package com.httpsms

class Constants {
    companion object {
        const val KEY_MESSAGE_ID = "KEY_MESSAGE_ID"
        const val KEY_MESSAGE_FROM = "KEY_MESSAGE_FROM"
        const val KEY_MESSAGE_TO = "KEY_MESSAGE_TO"
        const val KEY_MESSAGE_SIM = "KEY_MESSAGE_SIM"
        const val KEY_MESSAGE_CONTENT = "KEY_MESSAGE_CONTENT"
        const val KEY_MESSAGE_TIMESTAMP = "KEY_MESSAGE_TIMESTAMP"
        const val KEY_MESSAGE_REASON = "KEY_MESSAGE_REASON"
        const val KEY_MESSAGE_ENCRYPTED = "KEY_MESSAGE_ENCRYPTED"
        const val KEY_MESSAGE_ATTACHMENTS = "KEY_MESSAGE_ATTACHMENTS"


        const val KEY_HEARTBEAT_ID = "KEY_HEARTBEAT_ID"

        const val SIM1 = "SIM1"
        const val SIM2 = "SIM2"

        const val TIMESTAMP_PATTERN = "yyyy-MM-dd'T'HH:mm:ss.SSS'000000'ZZZZZ"

        // Outbound delivery is push-like without FCM: the foreground service
        // long-polls /v1/messages/outstanding/list?wait=… and the server holds
        // the request until a message is queued (or this many seconds elapse).
        // Must stay below the client read timeout in HttpSmsApiService.
        const val MESSAGE_LONG_POLL_WAIT_SECONDS = 25L

        const val MAX_MMS_ATTACHMENT_SIZE: Long = (3L * 1024 * 1024) / 2

        // Public-read bucket holding the OTA manifest and APK that
        // deploy.apk.sh publishes; the updater reads it without credentials.
        const val OTA_BASE_URL = "https://s3.bigt.ai/apps/httpsms/apk"
    }
}
