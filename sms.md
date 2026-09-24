# Enabling SMS Permissions for the HttpSms App

Package name: `com.httpsms`

HttpSms needs the runtime permissions `SEND_SMS`, `RECEIVE_SMS`, `READ_SMS`
and `RECEIVE_MMS`. It listens to `SMS_RECEIVED` (not `SMS_DELIVER`), so it does
**not** need to be the default SMS app to receive messages — sending only needs
`SEND_SMS`.

There are three ways to grant them. On Honor / MagicOS the UI often refuses to
grant SMS permissions, so the adb method is the reliable one.

---

## English

### Without adb — step-by-step (no computer needed)

Use this directly on the phone, in the Honor / MagicOS settings.

1. **Allow restricted settings** (required for sideloaded APKs on Android 13+):
   `Settings → Apps → HttpSms → ⋮ (three dots) → Allow restricted settings`.
   Without this, sensitive permissions cannot be granted at all.
2. **Set HttpSms as the default SMS app** — this is what unlocks the SMS
   permission group on Honor: `Settings → Apps → Default apps → SMS app → HttpSms`.
3. **Grant the SMS permissions:**
   `Settings → Apps → HttpSms → Permissions → SMS → Allow`
   (allow Send, Receive and Read).
   Alternative path: `Settings → Privacy → Permission manager → SMS → HttpSms → Allow`.
4. **If a toggle is greyed out:** open the item and pick **Allow**, or repeat
   steps 1–2. Some Honor builds need a reboot after step 2.
5. **Optional:** set the default SMS app back to the original app afterwards —
   HttpSms does not require it, it only listens to `SMS_RECEIVED`.
6. Close and reopen HttpSms so it picks up the permissions.

If the toggles stay locked, only adb can override the ROM policy (see Method 3).

### Method 1 — In-app (normal)

Open HttpSms. On launch it requests the SMS permissions (see
`MainActivity.kt`, `LoginActivity.kt`). Tap **Allow**.

### Method 2 — Android settings UI

`Settings → Apps → HttpSms → Permissions → SMS → Allow`.

On Honor / MagicOS this toggle is frequently missing, greyed out or refused.

### Method 3 — adb (definitive, bypasses Honor's UI refusal)

Requirements:
- USB debugging enabled on the phone
- The RSA **"Allow USB debugging?"** dialog accepted
- `adb devices` shows the phone as `device`

Check the connection:

```bash
adb devices -l
```

If adb is not installed, install Google's platform-tools (no root needed):

```bash
mkdir -p ~/android-sdk && cd ~/android-sdk
curl -fsSL -o platform-tools.zip https://dl.google.com/android/repository/platform-tools-latest-linux.zip
unzip -oq platform-tools.zip && rm platform-tools.zip
export PATH="$HOME/android-sdk/platform-tools:$PATH"
```

Grant the permissions (replace with your own path if needed):

```bash
export PATH="$HOME/android-sdk/platform-tools:$PATH"
adb shell pm grant com.httpsms android.permission.SEND_SMS
adb shell pm grant com.httpsms android.permission.RECEIVE_SMS
adb shell pm grant com.httpsms android.permission.READ_SMS
adb shell pm grant com.httpsms android.permission.RECEIVE_MMS
```

Verify:

```bash
adb shell dumpsys package com.httpsms | grep -iE "android.permission.(SEND_SMS|RECEIVE_SMS|READ_SMS|RECEIVE_MMS)"
```

Each must show `granted=true` under `User 0`. Then close and reopen the app.

`pm grant` only works for runtime (dangerous) permissions, which these are, and
the app must already be installed:

```bash
adb install -r out/HttpSms-debug-<sha>.apk
```

### Honor / MagicOS notes

The SMS permission group is often reserved for the default SMS app, which is
why the toggles are locked. If you prefer the UI:

1. **Allow restricted settings** (required for sideloaded APKs on Android 13+):
   `Settings → Apps → HttpSms → ⋮ → Allow restricted settings`.
2. **Make it the default SMS app:**
   `Settings → Apps → Default apps → SMS app → HttpSms`.
3. Then grant `Settings → Apps → HttpSms → Permissions → SMS → Allow`.
4. The default SMS app can be reverted afterwards.

If the UI still refuses, use the adb method above.

### Troubleshooting

- `adb devices` is empty / `lsusb` shows only MTP + Mass Storage and no ADB
  interface: USB debugging is not active. Enable **USB debugging** and
  **"Allow ADB debugging in charge only mode"**, then replug the cable and
  accept the RSA dialog. The USB interface list must contain `ff42`.
- App installed only in a secondary profile (e.g. Honor **Twin app**): grant
  per user with `adb shell pm grant --user <id> com.httpsms android.permission.SEND_SMS`.

---

## 中文

### 不用 adb —— 分步操作(无需电脑)

直接在手机上的荣耀 / MagicOS 设置里操作。

1. **允许受限设置**(Android 13+ 侧载 APK 必须):
   `设置 → 应用 → HttpSms → ⋮(三个点) → 允许受限设置`。
   不打开此项,敏感权限根本无法授予。
2. **把 HttpSms 设为默认短信应用** —— 这是荣耀上解锁短信权限组的关键:
   `设置 → 应用 → 默认应用 → 短信应用 → HttpSms`。
3. **授予短信权限:**
   `设置 → 应用 → HttpSms → 权限 → 短信 → 允许`
   (发送、接收、读取都允许)。
   备用路径:`设置 → 隐私 → 权限管理 → 短信 → HttpSms → 允许`。
4. **如果开关置灰:** 点进该项选择 **允许**,或重做第 1–2 步。
   部分荣耀版本在第 2 步后需要重启手机。
5. **可选:** 之后把默认短信应用改回原来的应用 —— HttpSms 并不需要它,
   它只监听 `SMS_RECEIVED`。
6. 关闭并重新打开 HttpSms,让它读取新权限。

如果开关仍被锁定,只有 adb 能绕过系统策略(见方法三)。

包名：`com.httpsms`

HttpSms 需要 `SEND_SMS`、`RECEIVE_SMS`、`READ_SMS`、`RECEIVE_MMS` 这几个运行时权限。
它监听的是 `SMS_RECEIVED`(不是 `SMS_DELIVER`),所以接收短信**不需要**成为默认短信应用，
发短信只需要 `SEND_SMS` 权限。

有三种授权方式。在荣耀 / MagicOS 上,系统 UI 经常会拒绝授予短信权限,所以 adb 方式最可靠。

### 方法一 —— 应用内(常规)

打开 HttpSms,启动时会请求短信权限(见 `MainActivity.kt`、`LoginActivity.kt`),
点击 **允许**。

### 方法二 —— 系统设置 UI

`设置 → 应用 → HttpSms → 权限 → 短信 → 允许`。

在荣耀 / MagicOS 上,这个开关经常缺失、置灰或被拒绝。

### 方法三 —— adb(最可靠,绕过荣耀 UI 拒绝)

前提条件:
- 手机上已开启 USB 调试
- 已接受 RSA **"允许 USB 调试吗?"** 弹窗
- `adb devices` 显示设备为 `device` 状态

检查连接:

```bash
adb devices -l
```

如未安装 adb,安装 Google 的 platform-tools(无需 root):

```bash
mkdir -p ~/android-sdk && cd ~/android-sdk
curl -fsSL -o platform-tools.zip https://dl.google.com/android/repository/platform-tools-latest-linux.zip
unzip -oq platform-tools.zip && rm platform-tools.zip
export PATH="$HOME/android-sdk/platform-tools:$PATH"
```

授予权限(按需替换路径):

```bash
export PATH="$HOME/android-sdk/platform-tools:$PATH"
adb shell pm grant com.httpsms android.permission.SEND_SMS
adb shell pm grant com.httpsms android.permission.RECEIVE_SMS
adb shell pm grant com.httpsms android.permission.READ_SMS
adb shell pm grant com.httpsms android.permission.RECEIVE_MMS
```

验证:

```bash
adb shell dumpsys package com.httpsms | grep -iE "android.permission.(SEND_SMS|RECEIVE_SMS|READ_SMS|RECEIVE_MMS)"
```

`User 0` 下的每一项都必须显示 `granted=true`。然后关闭并重新打开应用。

`pm grant` 只对运行时(危险)权限有效,以上这些都属于运行时权限;并且应用必须已安装:

```bash
adb install -r out/HttpSms-debug-<sha>.apk
```

### 荣耀 / MagicOS 说明

短信权限组通常只保留给默认短信应用,所以那些开关会被锁定。如果更想用 UI:

1. **允许受限设置**(Android 13+ 侧载 APK 必须):
   `设置 → 应用 → HttpSms → ⋮ → 允许受限设置`。
2. **设为默认短信应用:**
   `设置 → 应用 → 默认应用 → 短信应用 → HttpSms`。
3. 然后授予 `设置 → 应用 → HttpSms → 权限 → 短信 → 允许`。
4. 之后可以把默认短信应用改回去。

如果 UI 仍然拒绝,使用上面的 adb 方式。

### 故障排查

- `adb devices` 为空 / `lsusb` 只显示 MTP 和 Mass Storage,没有 ADB 接口:
  说明 USB 调试未生效。请开启 **USB 调试** 和 **"仅充电模式下允许 ADB 调试"**,
  然后重新插拔数据线并接受 RSA 弹窗。USB 接口列表必须包含 `ff42`。
- 应用只安装在副用户/分身(如荣耀 **应用分身/Twin app**)中:
  按用户分别授权:`adb shell pm grant --user <id> com.httpsms android.permission.SEND_SMS`。
