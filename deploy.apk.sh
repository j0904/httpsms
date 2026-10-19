#!/bin/bash
# deploy.apk.sh — build the HttpSms Android APK (docker) and upload it to MinIO.
#
#   ./deploy.apk.sh              # debug APK (auto-signed, sideloadable)
#   ./deploy.apk.sh --release    # release APK (signed only with android/app/keystore.properties)
#   ./deploy.apk.sh --skip-build # upload the APK already in out/
#   ./deploy.apk.sh --release --fdroid  # release APK + publish the self-hosted F-Droid repo
#
# Env:
#   S3_ENDPOINT  MinIO endpoint (default https://s3.bigt.ai)
#   S3_ACCESS_KEY / S3_SECRET_KEY (required)
#   S3_BUCKET    (default apps — the shared release bucket)
#   S3_PREFIX    object prefix inside the bucket (default httpsms/apk)
#   OUT_DIR      local APK dir (default ./out)
#   IMAGE        builder image name (default httpsms-apk)
#   MC_IMAGE     minio client image (default quay.io/minio/mc; use
#                ghcr.io/bigt-ai-platform/mc where quay.io is blocked)
#   PUBLIC_BASE_URL public base the OTA manifest's apk url is built from
#                (default $S3_ENDPOINT/$S3_BUCKET)
#   MANDATORY    true to mark this build mandatory in the OTA manifest
#   FDROID_DIR   local F-Droid workspace (default $OUT_DIR/fdroid)
#   FDROID_S3_PREFIX bucket prefix for the repo (default httpsms/fdroid)
#
# Uploads out/HttpSms-<type>-<sha>.apk plus the stable HttpSms.apk alias
# the docs link to, and writes httpsms-<type>-latest.json — the OTA manifest
# the in-app updater reads (android/.../Ota.kt). Both variants carry the
# certificate pinned in android/signing.sha256, so an APK installs over any
# build published by this script.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

BUILD_TYPE="debug"; SKIP_BUILD=0; FDROID=0
for a in "$@"; do
  case "$a" in
    --release) BUILD_TYPE=release ;;
    --skip-build) SKIP_BUILD=1 ;;
    --fdroid) FDROID=1 ;;
    --help|-h) sed -n '2,23p' "$0"; exit 0 ;;
    *) echo "unknown arg: $a" >&2; exit 1 ;;
  esac
done

# EU serving host: s3.bigt.ai is a transparent proxy that preserves Host, so
# SigV4 signatures (including presigned URLs) stay valid.
S3_ENDPOINT="${S3_ENDPOINT:-https://s3.bigt.ai}"
S3_BUCKET="${S3_BUCKET:-apps}"
S3_PREFIX="${S3_PREFIX:-httpsms/apk}"
S3_KEY_PREFIX="${S3_PREFIX:+$S3_PREFIX/}"
OUT_DIR="${OUT_DIR:-$ROOT/out}"
IMAGE="${IMAGE:-httpsms-apk}"
MC_IMAGE="${MC_IMAGE:-quay.io/minio/mc}"
PUBLIC_BASE_URL="${PUBLIC_BASE_URL:-$S3_ENDPOINT/$S3_BUCKET}"
: "${S3_ACCESS_KEY:?set S3_ACCESS_KEY}"
: "${S3_SECRET_KEY:?set S3_SECRET_KEY}"

APK_TASK="assembleDebug"
[ "$BUILD_TYPE" = release ] && APK_TASK="assembleRelease"
SHA="$(git rev-parse --short HEAD)"
OUT_APK="$OUT_DIR/HttpSms-$BUILD_TYPE-$SHA.apk"

if [ "$SKIP_BUILD" -eq 0 ]; then
  docker build -f android/Dockerfile --build-arg APK_TASK="$APK_TASK" -t "$IMAGE" .
  mkdir -p "$OUT_DIR"
  CID="$(docker create "$IMAGE")"
  docker cp "$CID:/app/app/build/outputs/apk/$BUILD_TYPE/app-$BUILD_TYPE.apk" "$OUT_APK"
  docker rm "$CID" >/dev/null
fi
[ -f "$OUT_APK" ] || { echo "missing $OUT_APK (run without --skip-build first)" >&2; exit 1; }
sha256sum "$OUT_APK"

# Read the version off the artifact when the SDK is around, so the manifest
# always describes the APK it points at; git (the same inputs gradle uses)
# is the fallback on machines without ANDROID_HOME.
VERSION_NAME="$(git rev-parse --short HEAD)"
VERSION_CODE="$(git rev-list --count HEAD)"
AAPT="$(ls "${ANDROID_HOME:-/nonexistent}"/build-tools/*/aapt 2>/dev/null | sort -V | tail -1 || true)"
if [ -n "$AAPT" ]; then
  BADGING="$("$AAPT" dump badging "$OUT_APK")"
  VERSION_NAME="$(printf '%s\n' "$BADGING" | sed -n "s/^package:.*versionName='\([^']*\)'.*/\1/p")"
  VERSION_CODE="$(printf '%s\n' "$BADGING" | sed -n "s/^package:.*versionCode='\([^']*\)'.*/\1/p")"
fi
[ -n "$VERSION_NAME" ] && [ -n "$VERSION_CODE" ] || { echo "cannot read version from $OUT_APK" >&2; exit 1; }
echo "version: $VERSION_NAME (code $VERSION_CODE)"

# Last gate before publishing: the APK has to carry the pinned certificate,
# otherwise no device can install it over the build it already runs.
PIN="$(tr -d ' \n' < "$ROOT/android/signing.sha256" 2>/dev/null || true)"
if [ -n "$AAPT" ] && [ -n "$PIN" ]; then
  APKSIGNER="$(dirname "$AAPT")/apksigner"
  GOT_PIN="$("$APKSIGNER" verify --print-certs "$OUT_APK" | awk '/certificate SHA-256 digest/{print $NF; exit}' | tr -d ':' | tr 'A-F' 'a-f')"
  if [ -z "$GOT_PIN" ]; then
    echo "warning: could not read the APK signer — pin not verified" >&2
  elif [ "$GOT_PIN" != "$PIN" ]; then
    echo "signer pin mismatch: built $GOT_PIN, pinned $PIN" >&2
    echo "refusing to publish — devices could not upgrade in place" >&2
    exit 1
  else
    echo "signer: pinned ($GOT_PIN)"
  fi
else
  echo "warning: no Android SDK (ANDROID_HOME) — signer pin not verified" >&2
fi

if [ "$BUILD_TYPE" = release ] && [ ! -f android/app/keystore.properties ]; then
  echo "warning: no android/app/keystore.properties — this release APK is UNSIGNED and will not install" >&2
fi

NAME="$(basename "$OUT_APK")"

# OTA manifest — the in-app updater fetches this straight from the (public)
# bucket and compares versionCode against the installed build.
APK_SHA="$(sha256sum "$OUT_APK" | awk '{print $1}')"
MANIFEST="httpsms-$BUILD_TYPE-latest.json"
printf '{"versionName":"%s","versionCode":%s,"url":"%s/%s","sha256":"%s","mandatory":%s}\n' \
  "$VERSION_NAME" "$VERSION_CODE" "${PUBLIC_BASE_URL%/}" "$S3_KEY_PREFIX$NAME" "$APK_SHA" "${MANDATORY:-false}" \
  > "$OUT_DIR/$MANIFEST"

docker run --rm \
  -e S3_ENDPOINT="$S3_ENDPOINT" -e S3_ACCESS_KEY="$S3_ACCESS_KEY" -e S3_SECRET_KEY="$S3_SECRET_KEY" \
  -e S3_BUCKET="$S3_BUCKET" -e S3_KEY_PREFIX="$S3_KEY_PREFIX" -e NAME="$NAME" -e MANIFEST="$MANIFEST" \
  -v "$OUT_DIR:/out:ro" --entrypoint /bin/sh "$MC_IMAGE" -c \
  'mc alias set up "$S3_ENDPOINT" "$S3_ACCESS_KEY" "$S3_SECRET_KEY" >/dev/null && \
   mc cp "/out/$NAME" "up/$S3_BUCKET/$S3_KEY_PREFIX$NAME" && \
   mc cp "/out/$NAME" "up/$S3_BUCKET/${S3_KEY_PREFIX}HttpSms.apk" && \
   mc cp "/out/$MANIFEST" "up/$S3_BUCKET/$S3_KEY_PREFIX$MANIFEST"' \
  || { echo "upload failed — check S3_* creds / MC_IMAGE ($MC_IMAGE)" >&2; exit 1; }

# --fdroid: also publish the release APK into a self-hosted F-Droid repo
# (workspace under $FDROID_DIR, listing text from ./fdroid/metadata/) and mirror
# it to $FDROID_S3_PREFIX/repo. The repo index is signed with the app's release
# key (android/app/keystore.properties).
publish_fdroid() {
  local apk="$1"
  local fd_dir="${FDROID_DIR:-$OUT_DIR/fdroid}"
  local s3_prefix="${FDROID_S3_PREFIX:-httpsms/fdroid}"
  local venv="${FDROID_VENV:-${XDG_CACHE_HOME:-$HOME/.cache}/fdroidserver-venv}"
  local props="$ROOT/android/app/keystore.properties"
  local base="${PUBLIC_BASE_URL:-${S3_ENDPOINT%/}/$S3_BUCKET}"
  local repo_url="${FDROID_REPO_URL:-${base%/}/$s3_prefix/repo}"

  # fdroidserver needs JDK tools (jar/keytool) and apksigner (via ANDROID_HOME).
  if ! command -v jar >/dev/null 2>&1; then
    local jdk=""
    if [ -n "${JAVA_HOME:-}" ] && [ -x "$JAVA_HOME/bin/jar" ]; then jdk="$JAVA_HOME"
    else for d in "$HOME"/.local/java-* /usr/lib/jvm/*; do [ -x "$d/bin/jar" ] && jdk="$d"; done; fi
    [ -n "$jdk" ] && PATH="$jdk/bin:$PATH" || { echo "fdroid: a JDK is required (jar on PATH or JAVA_HOME)" >&2; exit 1; }
  fi
  local aapt
  aapt="$(ls "${ANDROID_HOME:-/nonexistent}"/build-tools/*/aapt 2>/dev/null | sort -V | tail -1 || true)"
  [ -n "$aapt" ] || { echo "fdroid: ANDROID_HOME is required (aapt/apksigner)" >&2; exit 1; }

  if [ ! -x "$venv/bin/fdroid" ]; then
    echo "fdroid: creating fdroidserver venv at $venv (one-time, needs network)"
    python3 -m venv "$venv"
    "$venv/bin/pip" install -q --disable-pip-version-check fdroidserver
  fi
  local FDBIN="$venv/bin/fdroid"

  # Sign the index with the release key, read from keystore.properties.
  local keystore keyalias kspass
  keystore="${FDROID_KEYSTORE:-$(sed -n 's/^storeFile=//p' "$props" 2>/dev/null | head -1)}"
  keyalias="${FDROID_KEY_ALIAS:-$(sed -n 's/^keyAlias=//p' "$props" 2>/dev/null | head -1)}"
  kspass="${FDROID_KEY_PASS:-$(sed -n 's/^storePassword=//p' "$props" 2>/dev/null | head -1)}"
  [ -f "$keystore" ] || { echo "fdroid: keystore not found: ${keystore:-<none>} — see $props or FDROID_KEYSTORE" >&2; exit 1; }
  [ -n "$keyalias" ] && [ -n "$kspass" ] || { echo "fdroid: keystore alias/password missing — see $props or FDROID_KEY_ALIAS/FDROID_KEY_PASS" >&2; exit 1; }
  local keydname
  keydname="$(keytool -list -v -keystore "$keystore" -storepass "$kspass" -alias "$keyalias" 2>/dev/null | sed -n 's/^Owner: //p' | head -1)"
  [ -n "$keydname" ] || { echo "fdroid: cannot read the certificate DN for '$keyalias' in $keystore" >&2; exit 1; }

  local app_id version_code
  app_id="$("$aapt" dump badging "$apk" | sed -n "s/^package: name='\([^']*\)'.*/\1/p")"
  version_code="$("$aapt" dump badging "$apk" | sed -n "s/^package:.*versionCode='\([^']*\)'.*/\1/p")"
  [ -n "$app_id" ] && [ -n "$version_code" ] || { echo "fdroid: cannot read package/version from $apk" >&2; exit 1; }

  [ -d "$ROOT/fdroid/metadata" ] || { echo "fdroid: $ROOT/fdroid/metadata is missing" >&2; exit 1; }
  mkdir -p "$fd_dir/repo" "$fd_dir/metadata"
  cp -f "$ROOT"/fdroid/metadata/*.yml "$fd_dir/metadata/"
  cat > "$fd_dir/config.yml" <<EOF
repo_url: $repo_url
repo_name: ${FDROID_REPO_NAME:-httpSMS}
repo_description: ${FDROID_REPO_DESCRIPTION:-httpSMS Android builds (self-hosted), published by bigt.ai}
keystore: $keystore
repo_keyalias: $keyalias
keystorepass: $kspass
keypass: $kspass
keydname: $keydname
EOF
  chmod 600 "$fd_dir/config.yml"
  cp -f "$apk" "$fd_dir/repo/${app_id}_${version_code}.apk"

  ( cd "$fd_dir" && "$FDBIN" update )

  docker run --rm \
    -e S3_ENDPOINT="$S3_ENDPOINT" -e S3_ACCESS_KEY="$S3_ACCESS_KEY" -e S3_SECRET_KEY="$S3_SECRET_KEY" \
    -e S3_BUCKET="$S3_BUCKET" -e S3_PREFIX="$s3_prefix" \
    -v "$fd_dir/repo:/fdroid-repo:ro" --entrypoint /bin/sh "$MC_IMAGE" -c \
    'mc alias set up "$S3_ENDPOINT" "$S3_ACCESS_KEY" "$S3_SECRET_KEY" >/dev/null && \
     mc mirror --overwrite --remove /fdroid-repo "up/$S3_BUCKET/$S3_PREFIX/repo"' \
    || { echo "fdroid upload failed — check S3_* creds / MC_IMAGE ($MC_IMAGE)" >&2; exit 1; }

  local fp
  fp="$(keytool -list -v -keystore "$keystore" -storepass "$kspass" 2>/dev/null | sed -n 's/.*SHA256: //p' | head -1 || true)"
  echo "fdroid repo: $repo_url"
  echo "fdroid key : ${fp:-fingerprint unavailable}"
}

if [ "$FDROID" -eq 1 ]; then
  [ "$BUILD_TYPE" = release ] || { echo "--fdroid publishes release APKs only (not $BUILD_TYPE)" >&2; exit 1; }
  publish_fdroid "$OUT_APK"
fi

echo "uploaded: $PUBLIC_BASE_URL/$S3_KEY_PREFIX$NAME"
echo "uploaded: $PUBLIC_BASE_URL/${S3_KEY_PREFIX}HttpSms.apk"
echo "manifest: $PUBLIC_BASE_URL/$S3_KEY_PREFIX$MANIFEST ($VERSION_NAME / code $VERSION_CODE)"
