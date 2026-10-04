package com.httpsms

import org.junit.Assert.assertEquals
import org.junit.Test

class SmsTextRepairTest {
    @Test
    fun repairs_ascii_payload_declared_as_ucs2() {
        val garbled = "〲㘭㄰ⴰ㐠ㄶ㨵㤺㐴⁨慳⽨慶攠扥敮"
        assertEquals("026-10-04 16:59:44 has/have been", SmsTextRepair.repair(garbled))
    }

    @Test
    fun keeps_chinese_body() {
        val body = "祝你，2026顺顺顺，发大财"
        assertEquals(body, SmsTextRepair.repair(body))
    }

    @Test
    fun keeps_short_chinese_body() {
        val body = "好的"
        assertEquals(body, SmsTextRepair.repair(body))
    }

    @Test
    fun keeps_plain_ascii_body() {
        val body = "hello world"
        assertEquals(body, SmsTextRepair.repair(body))
    }

    @Test
    fun keeps_japanese_body() {
        val body = "あいうえおかきくけこ"
        assertEquals(body, SmsTextRepair.repair(body))
    }

    @Test
    fun keeps_body_with_emoji() {
        val body = "祝你 🎉 愿新一年好事连连"
        assertEquals(body, SmsTextRepair.repair(body))
    }

    @Test
    fun keeps_short_body() {
        assertEquals("", SmsTextRepair.repair(""))
        assertEquals("ij", SmsTextRepair.repair("ij"))
    }

    @Test
    fun repair_is_idempotent() {
        val repaired = SmsTextRepair.repair("〲㘭㄰ⴰ㐠ㄶ㨵㤺㐴⁨慳⽨慶攠扥敮")
        assertEquals(repaired, SmsTextRepair.repair(repaired))
    }
}
