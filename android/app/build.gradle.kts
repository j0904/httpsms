plugins {
    id("com.android.application")
    id("io.sentry.android.gradle") version "6.2.0"
}

import java.util.Properties
import java.io.FileInputStream

val keystoreProperties = Properties().apply {
    val propsFile = rootProject.file("app/keystore.properties")
    if (propsFile.exists()) {
        FileInputStream(propsFile).use { load(it) }
    }
}

val gitHash = providers.exec {
    commandLine("git", "rev-parse", "--short", "HEAD")
}.standardOutput.asText.map { it.trim() }

// The updater compares versionCode against the release manifest, so it has to
// increase on every build — the git hash in versionName is not ordered.
val gitCommitCount = providers.exec {
    commandLine("git", "rev-list", "--count", "HEAD")
}.standardOutput.asText.map { it.trim() }

android {
    compileSdk = 36

    defaultConfig {
        applicationId = "com.httpsms"
        minSdk = 28
        targetSdk = 36
        versionCode = gitCommitCount.getOrElse("1").toInt()
        versionName = gitHash.getOrElse("unknown")
        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    // One pinned certificate across every variant (android/signing.sha256): an
    // APK only installs over a build signed with the same key, so an
    // auto-generated debug key would make every update a manual reinstall.
    val appSigningConfig = if (keystoreProperties.isNotEmpty()) {
        signingConfigs.create("release") {
            storeFile = file(keystoreProperties["storeFile"] as String)
            storePassword = keystoreProperties["storePassword"] as String
            keyAlias = keystoreProperties["keyAlias"] as String
            keyPassword = keystoreProperties["keyPassword"] as String
        }
    } else {
        null
    }

    buildTypes {
        getByName("debug") {
            manifestPlaceholders["sentryEnvironment"] = "development"
            appSigningConfig?.let { signingConfig = it }
        }
        getByName("release") {
            manifestPlaceholders["sentryEnvironment"] = "production"
            isMinifyEnabled = false
            proguardFiles(getDefaultProguardFile("proguard-android-optimize.txt"), "proguard-rules.pro")
            signingConfig = appSigningConfig
        }
    }
    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_1_8
        targetCompatibility = JavaVersion.VERSION_1_8
    }
    namespace = "com.httpsms"

    buildFeatures {
        buildConfig = true
    }
}

dependencies {
    implementation("com.journeyapps:zxing-android-embedded:4.3.0")
    implementation("com.squareup.okhttp3:okhttp:5.3.2")
    implementation("com.jakewharton.timber:timber:5.0.1")
    implementation("androidx.preference:preference-ktx:1.2.1")
    implementation("androidx.work:work-runtime-ktx:2.11.1")
    implementation("androidx.core:core-ktx:1.18.0")
    implementation("androidx.cardview:cardview:1.0.0")
    implementation("com.beust:klaxon:5.6")
    implementation("androidx.appcompat:appcompat:1.7.1")
    implementation("org.apache.commons:commons-text:1.15.0")
    implementation("com.google.android.material:material:1.13.0")
    implementation("androidx.constraintlayout:constraintlayout:2.2.1")
    implementation("com.googlecode.libphonenumber:libphonenumber:9.0.26")
    implementation("com.klinkerapps:android-smsmms:5.2.6")
    testImplementation("junit:junit:4.13.2")
    androidTestImplementation("androidx.test.ext:junit:1.3.0")
    androidTestImplementation("androidx.test.espresso:espresso-core:3.7.0")
}
