package com.httpsms

import android.Manifest
import android.annotation.SuppressLint
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.os.PowerManager
import android.telephony.PhoneNumberUtils
import android.view.View
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.MutableLiveData
import androidx.work.Constraints
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.NetworkType
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import com.google.android.material.button.MaterialButton
import com.google.android.material.card.MaterialCardView
import com.google.android.material.progressindicator.LinearProgressIndicator
import com.httpsms.services.StickyNotificationService
import com.httpsms.worker.HeartbeatWorker
import timber.log.Timber
import java.time.Instant
import java.time.ZoneId
import java.time.ZoneOffset
import java.time.ZonedDateTime
import java.time.format.DateTimeFormatter
import java.util.*
import java.util.concurrent.TimeUnit
import android.provider.Settings as ProviderSettings


class MainActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        initTimber()

        redirectToLogin()

        setContentView(R.layout.activity_main)

        createChannel()

        setCardContent(this)
        registerListeners()

        startStickyNotification(this)
        scheduleHeartbeatWorker(this)
        setVersion()
        setHeartbeatListener(this)
        setSmsPermissionListener()
        setBatteryOptimizationListener()
        scheduleUpdateCheck()
    }

    override fun onStart() {
        super.onStart()
        requestPermissions(this)
    }

    override fun onResume() {
        super.onResume()
        Timber.d( "on activity resume")
        redirectToLogin()
        setCardContent(this)
        setSmsPermissionListener()
        setBatteryOptimizationListener()
    }

    private fun setVersion() {
        val appVersionView = findViewById<TextView>(R.id.mainAppVersion)
        appVersionView.text = getString(R.string.app_version, BuildConfig.VERSION_NAME)
    }

    private fun scheduleUpdateCheck() {
        // Let the first frame render before the network check, then prompt only
        // when a strictly newer build is published (Updater.check no-ops off-line).
        Handler(Looper.getMainLooper()).postDelayed({ checkForUpdate() }, 4000L)
    }

    private fun checkForUpdate() {
        if (isFinishing || isDestroyed) return
        Updater.check(this) { result ->
            if (result is Updater.CheckResult.Available) Updater.prompt(this, result.manifest)
        }
    }

    private fun setCardContent(context: Context) {
        val titleText = findViewById<TextView>(R.id.cardPhoneNumber)
        titleText.text = PhoneNumberUtils.formatNumber(Settings.getSIM1PhoneNumber(this), Locale.getDefault().country)
        if(!Settings.getActiveStatus(context, Constants.SIM1)) {
            titleText.setCompoundDrawables(null, null, null, null)
        }

        val titleTextSIM2 = findViewById<TextView>(R.id.cardPhoneNumberSIM2)
        titleTextSIM2.text = PhoneNumberUtils.formatNumber(Settings.getSIM2PhoneNumber(this), Locale.getDefault().country)
        if(!Settings.getActiveStatus(context, Constants.SIM2)) {
            titleTextSIM2.setCompoundDrawables(null, null, null, null)
        }

        setLastHeartbeatTimestamp(context)

        if(!Settings.isDualSIM(context)) {
            val sim2Card = findViewById<MaterialCardView>(R.id.mainPhoneCardSIM2)
            sim2Card.visibility = MaterialCardView.GONE
        }
    }

    private fun requestPermissions(context:Context) {
        Timber.d("requesting permissions")
        val requestPermissionLauncher = registerForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { permissions ->
            permissions.entries.forEach {
                Timber.d("${it.key} = ${it.value}")
                if (it.key == Manifest.permission.READ_CALL_LOG && !it.value) {
                    Timber.w("disabling incoming call events since for SIM1 and SIM2")
                    Settings.setIncomingCallEventsEnabled(context, Constants.SIM1, false)
                    Settings.setIncomingCallEventsEnabled(context, Constants.SIM2, false)
                }
            }
            setSmsPermissionListener()
        }

        var permissions = arrayOf(
            Manifest.permission.SEND_SMS,
            Manifest.permission.RECEIVE_SMS,
            Manifest.permission.READ_SMS
        )

        if(Build.VERSION.SDK_INT >= 33) {
            permissions += Manifest.permission.POST_NOTIFICATIONS
        }

        if(Settings.isIncomingCallEventsEnabled(context,Constants.SIM1) || Settings.isIncomingCallEventsEnabled(context,Constants.SIM2) ) {
            permissions += Manifest.permission.READ_CALL_LOG
            permissions += Manifest.permission.READ_PHONE_STATE
        }

        requestPermissionLauncher.launch(permissions)

        Timber.d("creating permissions launcher")
    }

    private fun scheduleHeartbeatWorker(context: Context) {
        val tag = "TAG_HEARTBEAT_WORKER"

        val constraints = Constraints.Builder()
            .setRequiredNetworkType(NetworkType.CONNECTED)
            .build()

        val heartbeatWorker =
            PeriodicWorkRequestBuilder<HeartbeatWorker>(15, TimeUnit.MINUTES)
                .setConstraints(constraints)
                .addTag(tag)
                .build()

        WorkManager
            .getInstance(context)
            .enqueueUniquePeriodicWork(tag, ExistingPeriodicWorkPolicy.KEEP, heartbeatWorker)

        Timber.d("finished scheduling heartbeat worker with ID [${heartbeatWorker.id}]")
    }

    private fun startStickyNotification(context: Context) {
        Timber.d("starting foreground service")
        if(!Settings.getActiveStatus(context, Constants.SIM1) && !Settings.getActiveStatus(context, Constants.SIM2)) {
            Timber.d("active status is false, not starting foreground service")
            return
        }
        val notificationIntent = Intent(context, StickyNotificationService::class.java)
        val service = context.startForegroundService(notificationIntent)
        Timber.d("foreground service started [${service?.className}]")
    }

    private fun initTimber() {
        if (Timber.treeCount > 1) {
            Timber.d("timber is already initialized with count [${Timber.treeCount}]")
            return
        }

        if(Settings.isDebugLogEnabled(this)) {
            Timber.plant(Timber.DebugTree())
            Timber.plant(LogzTree(this.applicationContext))
        }
    }

    private fun registerListeners() {
        findViewById<MaterialButton>(R.id.mainSettingsButton).setOnClickListener { onSettingsClick() }
    }

    private fun onSettingsClick() {
        Timber.d("settings button clicked")
        val switchActivityIntent = Intent(this, SettingsActivity::class.java)
        startActivity(switchActivityIntent)
    }

    private fun redirectToLogin():Boolean {
        if (Settings.isLoggedIn(this)) {
            return false
        }
        val switchActivityIntent = Intent(this, LoginActivity::class.java)
        startActivity(switchActivityIntent)
        return true
    }

    private fun setLastHeartbeatTimestamp(context: Context) {
        val refreshTimestampView = findViewById<TextView>(R.id.cardRefreshTime)
        val timestamp = Settings.getHeartbeatTimestamp(context)

        if (timestamp == 0.toLong()) {
            Timber.d("no heartbeat timestamp has been set")
            refreshTimestampView.text = "--"
            return
        }

        val timestampZdt = ZonedDateTime.ofInstant(Instant.ofEpochMilli(timestamp), ZoneOffset.UTC)
        val localTime = timestampZdt.withZoneSameInstant(ZoneId.systemDefault())
        Timber.d("heartbeat timestamp in UTC is [${timestampZdt}] and local is [$localTime]")

        refreshTimestampView.text = localTime.format(DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss"))

        if (Settings.isDualSIM(context)) {
            val refreshTimestampViewSIM2 = findViewById<TextView>(R.id.cardRefreshTimeSIM2)
            refreshTimestampViewSIM2.text = localTime.format(DateTimeFormatter.ofPattern("yyyy-MM-dd HH:mm:ss"))
        }
    }

    private fun createChannel() {
        // Create the NotificationChannel
        val name = getString(R.string.notification_channel_default)
        val descriptionText = getString(R.string.notification_channel_default)
        val importance = NotificationManager.IMPORTANCE_DEFAULT
        val mChannel = NotificationChannel(name, name, importance)
        mChannel.description = descriptionText
        // Register the channel with the system; you can't change the importance
        // or other notification behaviors after this
        val notificationManager = getSystemService(NOTIFICATION_SERVICE) as NotificationManager
        notificationManager.createNotificationChannel(mChannel)
    }

    @SuppressLint("BatteryLife")
    private fun setBatteryOptimizationListener() {
        val pm = getSystemService(POWER_SERVICE) as PowerManager
        val button = findViewById<MaterialButton>(R.id.batteryOptimizationButtonButton)
        if (!pm.isIgnoringBatteryOptimizations(packageName)) {
            button.visibility = View.VISIBLE
            button.setOnClickListener {
                val intent = Intent()
                intent.action = ProviderSettings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS
                intent.data = Uri.parse("package:$packageName")
                startActivity(intent)
            }
        } else {
            button.visibility = View.GONE
        }
        updatePermissionLayoutVisibility()
    }

    private fun setSmsPermissionListener() {
        val smsPermissions = arrayOf(
            Manifest.permission.SEND_SMS,
            Manifest.permission.RECEIVE_SMS,
            Manifest.permission.READ_SMS
        )
        val allGranted = smsPermissions.all {
            checkSelfPermission(it) == PackageManager.PERMISSION_GRANTED
        }

        val button = findViewById<MaterialButton>(R.id.smsPermissionButton)
        if (!allGranted) {
            button.visibility = View.VISIBLE
            button.setOnClickListener {
                // a twice-denied permission never prompts again — deep-link
                // straight to this app's system Settings page where the SMS
                // toggle lives (fully-qualified: Settings here is the app's
                // own preferences object, not android.provider.Settings).
                val intent = Intent(
                    android.provider.Settings.ACTION_APPLICATION_DETAILS_SETTINGS,
                    Uri.fromParts("package", packageName, null),
                )
                startActivity(intent)
            }
        } else {
            button.visibility = View.GONE
        }
        updatePermissionLayoutVisibility()
    }

    private fun updatePermissionLayoutVisibility() {
        val smsButton = findViewById<MaterialButton>(R.id.smsPermissionButton)
        val batteryButton = findViewById<MaterialButton>(R.id.batteryOptimizationButtonButton)
        val layout = findViewById<LinearLayout>(R.id.batteryOptimizationLinearLayout)

        if (smsButton.visibility == View.GONE && batteryButton.visibility == View.GONE) {
            layout.visibility = View.GONE
        } else {
            layout.visibility = View.VISIBLE
        }
    }

    private fun setHeartbeatListener(context: Context) {
        findViewById<MaterialButton>(R.id.mainHeartbeatButton).setOnClickListener{onHeartbeatClick(context)}
    }

    private fun onHeartbeatClick(context: Context) {
        Timber.d("heartbeat button clicked")
        val heartbeatButton = findViewById<MaterialButton>(R.id.mainHeartbeatButton)
        heartbeatButton.isEnabled = false

        val progressBar = findViewById<LinearProgressIndicator>(R.id.mainProgressIndicator)
        progressBar.visibility = View.VISIBLE

        val liveData = MutableLiveData<String?>()
        liveData.observe(this) { exception ->
            run {
                progressBar.visibility = View.INVISIBLE
                heartbeatButton.isEnabled = true

                if (exception != null) {
                    Timber.w("heartbeat sending failed with [$exception]")
                    Toast.makeText(context, exception, Toast.LENGTH_LONG).show()
                    return@run
                }
                Toast.makeText(context, "Heartbeat sent successfully", Toast.LENGTH_SHORT).show()

                setLastHeartbeatTimestamp(this)
            }
        }

        Thread {
            val charging = Settings.isCharging(applicationContext)
            var error: String? = null
            try {
                val phoneNumbers = mutableListOf<String>()
                phoneNumbers.add(Settings.getSIM1PhoneNumber(applicationContext))
                if (Settings.getActiveStatus(applicationContext, Constants.SIM2)) {
                    phoneNumbers.add(Settings.getSIM2PhoneNumber(applicationContext))
                }
                val isStored = HttpSmsApiService.create(context).storeHeartbeat(phoneNumbers.toTypedArray(), charging)
                if (!isStored) {
                    error = "Could not send heartbeat make sure the phone is connected to the internet"
                }
                Settings.setHeartbeatTimestampAsync(applicationContext, System.currentTimeMillis())
            } catch (exception: Exception) {
                Timber.e(exception)
                error = exception.javaClass.simpleName
            }
            liveData.postValue(error)
            Timber.d("finished sending pulse")
        }.start()
    }
}
