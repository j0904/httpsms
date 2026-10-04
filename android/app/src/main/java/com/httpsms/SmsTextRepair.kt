package com.httpsms

/**
 * Repairs an SMS body that was decoded with the wrong alphabet.
 *
 * Some aggregators put GSM/ASCII bytes on the wire while declaring UCS-2 (DCS 0x08).
 * The platform then pairs those bytes into UTF-16 code units, so a body like
 * "2026-10-04 16:59:44 has/have been" reaches the app as "〲㘭㄰ …".
 *
 * The repair only fires when every code unit maps onto printable ASCII and the
 * recovered text reads like a message, so normal Chinese/Japanese/Korean bodies
 * are never rewritten.
 */
object SmsTextRepair {
    private const val MIN_LENGTH = 4
    private const val MIN_TEXT_RATIO = 0.9
    private const val FORBIDDEN_CHARS = "]`{}|\\~^<>"

    fun repair(body: String): String {
        if (body.length < MIN_LENGTH) {
            return body
        }

        val payload = ByteArray(body.length * 2)
        for (i in body.indices) {
            val codeUnit = body[i].code
            payload[i * 2] = (codeUnit shr 8).toByte()
            payload[i * 2 + 1] = (codeUnit and 0xff).toByte()
        }
        if (!payload.all { isPayloadByte(it) }) {
            return body
        }

        val candidate = String(payload, Charsets.US_ASCII)
        if (candidate.any { it in FORBIDDEN_CHARS }) {
            return body
        }
        if (candidate.count { isTextChar(it) }.toDouble() / candidate.length < MIN_TEXT_RATIO) {
            return body
        }
        if (!hasLongRun(candidate)) {
            return body
        }
        return candidate
    }

    private fun isPayloadByte(value: Byte): Boolean {
        val octet = value.toInt() and 0xff
        return octet in 0x09..0x0d || octet in 0x20..0x7e
    }

    private fun isTextChar(value: Char): Boolean {
        return value in 'a'..'z' || value in 'A'..'Z' || value in '0'..'9' || value in " \t.,:;!?/'\"-/%"
    }

    private fun hasLongRun(text: String): Boolean {
        var letters = 0
        var digits = 0
        for (value in text) {
            when {
                value in 'a'..'z' || value in 'A'..'Z' -> {
                    letters++
                    digits = 0
                }

                value in '0'..'9' -> {
                    digits++
                    letters = 0
                }

                else -> {
                    letters = 0
                    digits = 0
                }
            }
            if (letters >= 3 || digits >= 3) {
                return true
            }
        }
        return false
    }
}
