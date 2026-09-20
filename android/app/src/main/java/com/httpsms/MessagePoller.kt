package com.httpsms

import android.content.Context
import androidx.work.Constraints
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequest
import androidx.work.WorkManager
import androidx.work.workDataOf
import timber.log.Timber

// Without push notifications the phone long-polls for outstanding messages and
// enqueues one SendSmsWorker per message ID. Claiming happens server-side in
// GetOutstanding, so overlapping polls cannot double-send.
object MessagePoller {
    fun pollOutstanding(context: Context, waitSeconds: Long = 0) {
        val messageIDs = try {
            HttpSmsApiService.create(context).getOutstandingMessageIDs(waitSeconds)
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
            val inputData = workDataOf(Constants.KEY_MESSAGE_ID to messageID)
            val work = OneTimeWorkRequest
                .Builder(SendSmsWorker::class.java)
                .setConstraints(constraints)
                .setInputData(inputData)
                .addTag(messageID)
                .build()

            WorkManager
                .getInstance(context)
                .enqueue(work)
        }
    }
}
