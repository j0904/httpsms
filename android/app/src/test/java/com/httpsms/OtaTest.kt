package com.httpsms

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class OtaTest {
    private val json = """
        {"versionName":"4241169e","versionCode":42,
         "url":"https://s3.bigt.ai/apps/httpsms/apk/HttpSms-debug-4241169e.apk",
         "sha256":"241158da4a5426401bd9bb83198edc8dcaf5578fe896826d3c464e4a2fabe171",
         "mandatory":true}
    """.trimIndent()

    @Test
    fun parsesManifest() {
        val manifest = Ota.parse(json)
        assertNotNull(manifest)
        assertEquals("4241169e", manifest!!.versionName)
        assertEquals(42, manifest.versionCode)
        assertEquals("https://s3.bigt.ai/apps/httpsms/apk/HttpSms-debug-4241169e.apk", manifest.url)
        assertEquals("241158da4a5426401bd9bb83198edc8dcaf5578fe896826d3c464e4a2fabe171", manifest.sha256)
        assertTrue(manifest.mandatory)
    }

    @Test
    fun rejectsUnusablePayloads() {
        assertNull(Ota.parse(""))
        assertNull(Ota.parse("not json"))
        assertNull(Ota.parse("{}"))
        assertNull(Ota.parse("""{"versionCode":42,"url":""}"""))
        assertNull(Ota.parse("""{"versionCode":0,"url":"https://x/apk"}"""))
        assertNull(Ota.parse("""{"versionCode":-1,"url":"https://x/apk"}"""))
        assertNull(Ota.parse("""{"versionCode":"42","url":"https://x/apk"}"""))
    }

    @Test
    fun hasUpdateComparesVersionCodeOnly() {
        val manifest = Ota.parse(json)
        assertTrue(Ota.hasUpdate(manifest, 41))
        assertFalse(Ota.hasUpdate(manifest, 42))
        assertFalse(Ota.hasUpdate(manifest, 43))
        assertFalse(Ota.hasUpdate(null, 1))
    }

    @Test
    fun manifestUrlFollowsBuildType() {
        assertEquals(
            "https://s3.bigt.ai/apps/httpsms/apk/httpsms-debug-latest.json",
            Ota.manifestUrl("https://s3.bigt.ai/apps/httpsms/apk", isDebug = true),
        )
        assertEquals(
            "https://s3.bigt.ai/apps/httpsms/apk/httpsms-release-latest.json",
            Ota.manifestUrl("https://s3.bigt.ai/apps/httpsms/apk/", isDebug = false),
        )
    }
}
