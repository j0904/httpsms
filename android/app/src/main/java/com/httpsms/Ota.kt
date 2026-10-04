package com.httpsms

import com.beust.klaxon.Klaxon

/**
 * Release manifest published next to an APK by `deploy.apk.sh`.
 */
data class OtaManifest(
    val versionName: String,
    val versionCode: Int,
    val url: String,
    val sha256: String,
    val mandatory: Boolean,
)

/**
 * Pure manifest helpers — no Android types, so they are unit testable.
 *
 * `deploy.apk.sh` uploads `httpsms-<type>-latest.json` into the public bucket
 * beside the APK: `{versionName, versionCode, url, sha256, mandatory}`. The app
 * fetches it, compares `versionCode` against the installed build and hands the
 * `url` + `sha256` to [Updater], which downloads, verifies and installs it.
 * Only `versionCode` decides — it must strictly increase per build.
 */
object Ota {

    fun manifestName(isDebug: Boolean): String =
        if (isDebug) "httpsms-debug-latest.json" else "httpsms-release-latest.json"

    fun manifestUrl(base: String, isDebug: Boolean): String =
        "${base.trimEnd('/')}/${manifestName(isDebug)}"

    /** Null when the payload is not a usable release (never trust a bad object). */
    fun parse(json: String): OtaManifest? = try {
        val manifest = Klaxon().parse<OtaManifest>(json) ?: return null
        if (manifest.url.isEmpty() || manifest.versionCode <= 0) null else manifest
    } catch (e: Exception) {
        null
    }

    fun hasUpdate(manifest: OtaManifest?, currentVersionCode: Long): Boolean =
        manifest != null && manifest.versionCode.toLong() > currentVersionCode
}
