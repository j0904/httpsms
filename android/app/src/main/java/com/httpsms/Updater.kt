package com.httpsms

import android.app.Activity
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.content.pm.PackageInstaller
import android.os.Build
import android.os.Handler
import android.os.Looper
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import okhttp3.OkHttpClient
import okhttp3.Request
import timber.log.Timber
import java.io.File
import java.io.FileInputStream
import java.security.MessageDigest
import java.util.concurrent.Executors

/**
 * OTA updater: fetch the release manifest, compare its versionCode against the
 * installed build, then download the APK, verify its sha256 and hand it to
 * [PackageInstaller]. Session status arrives in [UpdateReceiver]; a build that
 * needs the user's consent opens the system confirm activity from there.
 *
 * The bucket is public-read, so no gateway is involved and only the manifest's
 * `url` + `sha256` are trusted.
 */
object Updater {

    /** Session install reads/writes take longer than a UI round trip. */
    private const val WRITE_TIMEOUT_SECONDS = 120L

    private val client = OkHttpClient.Builder()
        .connectTimeout(15, java.util.concurrent.TimeUnit.SECONDS)
        .readTimeout(WRITE_TIMEOUT_SECONDS, java.util.concurrent.TimeUnit.SECONDS)
        .build()

    private val executor = Executors.newSingleThreadExecutor()
    private val mainHandler = Handler(Looper.getMainLooper())

    fun currentVersionCode(context: Context): Long {
        val info = context.packageManager.getPackageInfo(context.packageName, 0)
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            info.longVersionCode
        } else {
            @Suppress("DEPRECATION")
            info.versionCode.toLong()
        }
    }

    /** Manifest this build reads: debug builds track the debug channel. */
    fun manifestUrl(): String = Ota.manifestUrl(Constants.OTA_BASE_URL, BuildConfig.DEBUG)

    /**
     * Outcome of a manifest check: an update, a verified "you are current", or
     * a check that could not be made (offline, malformed manifest) — the last
     * must not be reported to the user as "up to date".
     */
    sealed interface CheckResult {
        data class Available(val manifest: OtaManifest) : CheckResult
        object UpToDate : CheckResult
        object Unavailable : CheckResult
    }

    /**
     * Check the manifest on a background thread. The callback runs on the main
     * thread.
     */
    fun check(context: Context, onResult: (CheckResult) -> Unit) {
        executor.execute {
            val appContext = context.applicationContext
            val result = fetch(appContext)
            mainHandler.post { onResult(result) }
        }
    }

    /** Background fetch + compare. */
    private fun fetch(context: Context): CheckResult = try {
        val request = Request.Builder()
            .url(manifestUrl())
            .header("Accept", "application/json")
            .build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) {
                Timber.w("OTA manifest HTTP %d", response.code)
                return CheckResult.Unavailable
            }
            val body = response.body?.string()
            val manifest = if (body == null) null else Ota.parse(body)
            if (manifest == null) {
                Timber.w("OTA manifest is not a usable release")
                return CheckResult.Unavailable
            }
            if (!Ota.hasUpdate(manifest, currentVersionCode(context))) return CheckResult.UpToDate
            CheckResult.Available(manifest)
        }
    } catch (e: Exception) {
        Timber.w(e, "OTA check failed")
        CheckResult.Unavailable
    }

    /**
     * Ask before a non-mandatory update; a mandatory one installs straight away
     * (the system consent sheet still appears — the app is not a device owner).
     */
    fun prompt(activity: Activity, manifest: OtaManifest) {
        if (activity.isFinishing || activity.isDestroyed) return
        if (manifest.mandatory) {
            install(activity.applicationContext, manifest)
            return
        }
        MaterialAlertDialogBuilder(activity)
            .setTitle(R.string.update_available_title)
            .setMessage(
                activity.getString(
                    R.string.update_available_message,
                    manifest.versionName,
                    BuildConfig.VERSION_NAME,
                )
            )
            .setPositiveButton(R.string.update_install) { _, _ ->
                install(activity.applicationContext, manifest)
            }
            .setNegativeButton(R.string.update_later, null)
            .show()
    }

    /** Download + verify + install on a background thread. */
    fun install(context: Context, manifest: OtaManifest) {
        executor.execute {
            try {
                val apk = download(context, manifest.url)
                val actual = sha256(apk)
                if (!manifest.sha256.equals(actual, ignoreCase = true)) {
                    apk.delete()
                    Timber.w("OTA sha256 mismatch: expected %s got %s", manifest.sha256, actual)
                    showToast(context, R.string.update_sha_mismatch)
                    return@execute
                }
                session(context, apk)
            } catch (e: Exception) {
                Timber.e(e, "OTA install failed")
                showToast(context, R.string.update_download_failed)
            }
        }
    }

    private fun download(context: Context, url: String): File {
        val request = Request.Builder().url(url).build()
        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) throw IllegalStateException("HTTP ${response.code}")
            val dir = File(context.cacheDir, "ota")
            dir.mkdirs()
            val apk = File(dir, "update.apk")
            response.body?.byteStream()?.use { input ->
                apk.outputStream().use { output -> input.copyTo(output) }
            } ?: throw IllegalStateException("empty response body")
            return apk
        }
    }

    private fun sha256(file: File): String {
        val digest = MessageDigest.getInstance("SHA-256")
        FileInputStream(file).use { input ->
            val buffer = ByteArray(8192)
            var read = input.read(buffer)
            while (read > 0) {
                digest.update(buffer, 0, read)
                read = input.read(buffer)
            }
        }
        return digest.digest().joinToString("") { "%02x".format(it) }
    }

    private fun session(context: Context, apk: File) {
        val installer = context.packageManager.packageInstaller
        val params = PackageInstaller.SessionParams(PackageInstaller.SessionParams.MODE_FULL_INSTALL)
        val sessionId = installer.createSession(params)
        installer.openSession(sessionId).use { session ->
            session.openWrite("update", 0, apk.length()).use { output ->
                FileInputStream(apk).use { input -> input.copyTo(output) }
            }
            session.commit(statusSender(context, sessionId))
        }
        Timber.i("OTA install session %d committed", sessionId)
    }

    private fun statusSender(context: Context, sessionId: Int): android.content.IntentSender {
        val intent = Intent(context, UpdateReceiver::class.java)
            .putExtra("sessionId", sessionId)
        var flags = PendingIntent.FLAG_UPDATE_CURRENT
        // The installer writes EXTRA_STATUS into this intent, so it must stay
        // mutable on S+ — an immutable receiver never receives the extras.
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            flags = flags or PendingIntent.FLAG_MUTABLE
        }
        return PendingIntent.getBroadcast(context, sessionId, intent, flags).intentSender
    }

    private fun showToast(context: Context, message: Int) {
        mainHandler.post {
            android.widget.Toast.makeText(context, message, android.widget.Toast.LENGTH_LONG).show()
        }
    }
}
