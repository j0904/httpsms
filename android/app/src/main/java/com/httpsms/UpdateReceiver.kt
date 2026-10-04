package com.httpsms

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.pm.PackageInstaller
import android.widget.Toast
import timber.log.Timber

/**
 * Receives the PackageInstaller session status. A separate receiver is required
 * because the installer's PendingIntent targets a receiver component rather than
 * an activity, and the consent sheet it hands back has to be launched from here.
 */
class UpdateReceiver : BroadcastReceiver() {

    override fun onReceive(context: Context, intent: Intent) {
        val status = intent.getIntExtra(PackageInstaller.EXTRA_STATUS, PackageInstaller.STATUS_FAILURE)

        if (status == PackageInstaller.STATUS_PENDING_USER_ACTION) {
            @Suppress("DEPRECATION")
            val confirm = intent.getParcelableExtra<Intent>(Intent.EXTRA_INTENT)
            confirm?.let {
                it.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                context.startActivity(it)
            }
            return
        }

        val sessionId = intent.getIntExtra("sessionId", -1)
        if (status == PackageInstaller.STATUS_SUCCESS) {
            Timber.i("OTA install session %d succeeded", sessionId)
            return
        }

        Timber.w("OTA install session %d failed with status %d", sessionId, status)
        Toast.makeText(context, R.string.update_download_failed, Toast.LENGTH_LONG).show()
    }
}
