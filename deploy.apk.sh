#!/bin/bash
# deploy.apk.sh — build the HttpSms Android APK (docker) and upload it to MinIO.
#
#   ./deploy.apk.sh              # debug APK (auto-signed, sideloadable)
#   ./deploy.apk.sh --release    # release APK (signed only with android/app/keystore.properties)
#   ./deploy.apk.sh --skip-build # upload the APK already in out/
#
# Env:
#   S3_ENDPOINT  MinIO endpoint (default https://minio-s1001.bigt.ai)
#   S3_ACCESS_KEY / S3_SECRET_KEY (required)
#   S3_BUCKET    (default aifeeds-content)
#   S3_PREFIX    object prefix (default httpsms)
#   OUT_DIR      local APK dir (default ./out)
#   IMAGE        builder image name (default httpsms-apk)
#
# Uploads out/HttpSms-<type>-<sha>.apk plus the stable httpsms/HttpSms.apk
# alias the docs link to. Debug and release signatures differ: switching
# between them on a phone requires uninstalling first.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

BUILD_TYPE="debug"; SKIP_BUILD=0
for a in "$@"; do
  case "$a" in
    --release) BUILD_TYPE=release ;;
    --skip-build) SKIP_BUILD=1 ;;
    --help|-h) sed -n '2,20p' "$0"; exit 0 ;;
    *) echo "unknown arg: $a" >&2; exit 1 ;;
  esac
done

S3_ENDPOINT="${S3_ENDPOINT:-https://minio-s1001.bigt.ai}"
S3_BUCKET="${S3_BUCKET:-aifeeds-content}"
S3_PREFIX="${S3_PREFIX:-httpsms}"
OUT_DIR="${OUT_DIR:-$ROOT/out}"
IMAGE="${IMAGE:-httpsms-apk}"
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

if [ "$BUILD_TYPE" = release ] && [ ! -f android/app/keystore.properties ]; then
  echo "warning: no android/app/keystore.properties — this release APK is UNSIGNED and will not install" >&2
fi

NAME="$(basename "$OUT_APK")"
docker run --rm \
  -e S3_ENDPOINT="$S3_ENDPOINT" -e S3_ACCESS_KEY="$S3_ACCESS_KEY" -e S3_SECRET_KEY="$S3_SECRET_KEY" \
  -e S3_BUCKET="$S3_BUCKET" -e S3_PREFIX="$S3_PREFIX" -e NAME="$NAME" \
  -v "$OUT_DIR:/out:ro" --entrypoint /bin/sh quay.io/minio/mc -c \
  'mc alias set up "$S3_ENDPOINT" "$S3_ACCESS_KEY" "$S3_SECRET_KEY" >/dev/null && \
   mc cp "/out/$NAME" "up/$S3_BUCKET/$S3_PREFIX/$NAME"' \
  || { echo "upload failed — check S3_* creds" >&2; exit 1; }
docker run --rm \
  -e S3_ENDPOINT="$S3_ENDPOINT" -e S3_ACCESS_KEY="$S3_ACCESS_KEY" -e S3_SECRET_KEY="$S3_SECRET_KEY" \
  -e S3_BUCKET="$S3_BUCKET" -e S3_PREFIX="$S3_PREFIX" -e NAME="$NAME" \
  -v "$OUT_DIR:/out:ro" --entrypoint /bin/sh quay.io/minio/mc -c \
  'mc alias set up "$S3_ENDPOINT" "$S3_ACCESS_KEY" "$S3_SECRET_KEY" >/dev/null && \
   mc cp "up/$S3_BUCKET/$S3_PREFIX/$NAME" "up/$S3_BUCKET/$S3_PREFIX/HttpSms.apk"' \
  || { echo "stable-alias copy failed" >&2; exit 1; }

echo "uploaded: $S3_ENDPOINT/$S3_BUCKET/$S3_PREFIX/$NAME"
echo "uploaded: $S3_ENDPOINT/$S3_BUCKET/$S3_PREFIX/HttpSms.apk"
