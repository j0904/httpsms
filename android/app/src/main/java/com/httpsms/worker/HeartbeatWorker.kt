package com.httpsms.worker

import android.content.Context
import androidx.work.Constraints
import androidx.work.Data
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequest
import androidx.work.WorkManager
import androidx.work.Worker
import androidx.work.WorkerParameters
import androidx.work.workDataOf
import com.httpsms.Constants
import com.httpsms.HttpSmsApiService
import com.httpsms.SendSmsWorker
import com.httpsms.Settings
import timber.log.Timber

class HeartbeatWorker(appContext: Context, workerParams: WorkerParameters) : Worker(appContext, workerParams) {
    override fun doWork(): Result {
        Timber.d("executing heartbeat worker")
        if (!Settings.isLoggedIn(applicationContext)) {
            Timber.w("user is not logged in, stopping processing")
            return Result.failure()
        }

        val phoneNumbers = mutableListOf<String>()
        if (Settings.getActiveStatus(applicationContext, Constants.SIM1)) {
            phoneNumbers.add(Settings.getSIM1PhoneNumber(applicationContext))
        }
        if (Settings.getActiveStatus(applicationContext, Constants.SIM2)) {
            phoneNumbers.add(Settings.getSIM2PhoneNumber(applicationContext))
        }

        if (phoneNumbers.isEmpty()) {
            Timber.w("both [SIM1] and [SIM2] are inactive stopping processing.")
            return Result.success()
        }

        try{
            HttpSmsApiService.create(applicationContext).storeHeartbeat(phoneNumbers.toTypedArray(), Settings.isCharging(applicationContext))
            Timber.d("finished sending heartbeats to server")

            Settings.setHeartbeatTimestampAsync(applicationContext, System.currentTimeMillis())
            Timber.d("Set the heartbeat timestamp")

            pollOutstandingMessages()
        } catch (exception: Exception) {
            Timber.e(exception, "Failed to send [${phoneNumbers.joinToString()}] heartbeats to server")
            return Result.failure()
        }

        return Result.success()
    }

    // Without push notifications the phone polls for outstanding messages and
    // enqueues one SendSmsWorker per message ID. Claiming happens server-side
    // in GetOutstanding, so overlapping polls cannot double-send.
    private fun pollOutstandingMessages() {
        val messageIDs = try {
            HttpSmsApiService.create(applicationContext).getOutstandingMessageIDs()
        } catch (exception: Exception) {
            Timber.e(exception, "Failed to fetch outstanding message IDs")
            return
        }
        if (messageIDs.isEmpty()) {
            return
        }
        Timber.d("fetched [${messageIDs.size}] outstanding message IDs")

        val constraints = Constraints.Builder()
            .setRequiredNetworkType(NetworkType.CONNECTED)
            .build()

        for (messageID in messageIDs) {
            val inputData: Data = workDataOf(Constants.KEY_MESSAGE_ID to messageID)
            val work = OneTimeWorkRequest
                .Builder(SendSmsWorker::class.java)
                .setConstraints(constraints)
                .setInputData(inputData)
                .addTag(messageID)
                .build()

            WorkManager
                .getInstance(applicationContext)
                .enqueue(work)
        }
    }
}
