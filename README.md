# httpSMS

[![Web](https://github.com/NdoleStudio/httpsms/actions/workflows/web.yml/badge.svg)](https://github.com/NdoleStudio/httpsms/actions/workflows/web.yml)
[![API](https://github.com/NdoleStudio/httpsms/actions/workflows/api.yml/badge.svg)](https://github.com/NdoleStudio/httpsms/actions/workflows/api.yml)
[![GitHub contributors](https://img.shields.io/github/contributors/NdoleStudio/httpsms)](https://github.com/NdoleStudio/httpsms/graphs/contributors)
[![GitHub license](https://img.shields.io/github/license/NdoleStudio/httpsms?color=brightgreen)](https://github.com/NdoleStudio/httpsms/blob/master/LICENSE)
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-2.1-4baaaa.svg)](CODE_OF_CONDUCT.md)
[![Scrutinizer Code Quality](https://scrutinizer-ci.com/g/NdoleStudio/httpsms/badges/quality-score.png?b=main)](https://scrutinizer-ci.com/g/NdoleStudio/httpsms/?branch=main)
[![Better Stack Badge](https://uptime.betterstack.com/status-badges/v1/monitor/ldl5.svg)](https://uptime.betterstack.com/?utm_source=status_badge)
[![Sponsors](https://img.shields.io/github/sponsors/ndolestudio?logo=github)](https://github.com/sponsors/ndolestudio)
[![Discord](https://img.shields.io/discord/1095778291488653372?label=Discord)](https://discord.gg/kGk8HVqeEZ)

[httpSMS](https://httpsms.com) is a service that lets you use your Android phone as an SMS Gateway to send and receive SMS messages.
You make a request to a simple HTTP API and it triggers your Android phone to send an SMS. SMS messages received on your android phone can also be forwarded to your webhook endpoint.

Quick Start Guide 👉 [https://docs.httpsms.com](https://docs.httpsms.com)

<img width="1115" alt="header" src="https://user-images.githubusercontent.com/4196457/194767449-f12d84a0-22f1-4787-afb2-17398fb459f6.png">

## Table Of Contents

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->

- [Why?](#why)
- [Web UI](#web-ui)
- [API](#api)
- [Android App](#android-app)
- [Chat/forum](#chatforum)
- [Features](#features)
  - [End-to-end Encryption](#end-to-end-encryption)
  - [Webhook](#webhook)
  - [Back Pressure](#back-pressure)
  - [Message Expiration](#message-expiration)
- [API Clients](#api-clients)
- [Flows](#flows)
  - [Sending an SMS Message](#sending-an-sms-message)
- [Self Host Setup - Docker](#self-host-setup---docker)
  - [1. Setup SMTP Email service](#1-setup-smtp-email-service)
  - [2. Setup Cloudflare Turnstile](#2-setup-cloudflare-turnstile)
  - [3. Download the code](#3-download-the-code)
  - [4. Setup the environment variables](#4-setup-the-environment-variables)
  - [5. Build and Run](#5-build-and-run)
  - [6. Create the System User](#6-create-the-system-user)
  - [7. Build the Android App.](#7-build-the-android-app)
- [Integration Testing](#integration-testing)
- [License](#license)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Why?

I'm originally from Cameroon and I wanted an automated way to send and receive SMS messages using an API.
Unfortunately many countries don't support the ability to buy virtual phone numbers, and I could not find a good ready-made
solution that could help me send/receive SMS messages using a mobile phone using an intuitive http API.

## Web UI

The web interface https://httpsms.com is built using [Nuxt](https://nuxtjs.org/) and [Vuetify](https://vuetifyjs.com/en/).
It is a single page application (deployed to Cloudflare Pages). The source code is in the [web](./web) directory

## API

The API https://api.httpsms.com is built using [Fiber](https://gofiber.io/), Go and [CockroachDB](https://www.cockroachlabs.com/) for the database.
It rus as a serverless application on Google Cloud Run. The API documentation can be found here https://api.httpsms.com/index.html

```go
// Sending an SMS Message using Go
client := htpsms.New(htpsms.WithAPIKey(/* API Key from https://httpsms.com/settings */))

client.Messages.Send(context.Background(), &httpsms.MessageSendParams{
    Content: "This is a sample text message",
    From:    "+18005550199",
    To:      "+18005550100",
})
```

## Android App

[The Android App](https://apk.httpsms.com/HttpSms.apk) is a native application built using Kotlin with material design principles.
This app must be installed on an Android phone before you can start sending and receiving SMS messages.

[<img src=".github/ghbadge.png" alt="Get it on GitHub" height="80">](https://github.com/NdoleStudio/httpsms/releases/)

## Chat/forum

There are a few ways to get in touch with me and/or the rest of the community. Feel free to use any of these methods. Whatever
works best for you:

- [Discord server](https://discord.gg/kGk8HVqeEZ) - direct chat with the community
- [GitHub issues](https://github.com/NdoleStudio/httpsms/issues) - questions, features, bugs

## Features

### End-to-end Encryption

You can encrypt your messages end-to-end ysubg the military grade [AES-256 encryption](https://en.wikipedia.org/wiki/Advanced_Encryption_Standard)
algorithm. Your encryption key is stored only on our mobile phone so the even the server won't have any way to view the
content of your SMS messages which are sent and received on your Android phone.

### Webhook

If you want to build advanced integrations, we support webhooks. The httpSMS platform can forward SMS messages received
on the android phone to your server using a callback URL which you provide.

### Back Pressure

In-order not to abuse the SMS API on android, you can set a rate limit e.g 3 messages per minute. Such that even if you
call the API to send messages to 100 people, It will only send the messages at a rate of 3 messages per minute.

### Message Expiration

Sometimes it happens that the phone doesn't pick up the queued message in time and I can't send the SMS message. It is
possible to set a timeout for which a message is valid and if a message becomes expired after the timeout elapses, you
will be notified.

## API Clients

- [x] Go: https://github.com/NdoleStudio/httpsms-go
- [x] JavaScript/TypeScript: https://github.com/NdoleStudio/httpsms-node

## Flows

### Sending an SMS Message

```mermaid
sequenceDiagram
User->>+httpSMS API: Call /v1/messages/send API
httpSMS API-->>+Delivery Queue: Queue the message for the phone
httpSMS API-->>-User: Respond with 202 (Accepted)
Delivery Queue-->>+httpSMS API: [Async] Mark the phone's queue as having work
httpSMS API-->>-Android App: Hold the message (the phone long-polls)
Android App-->>httpSMS API: [Async] Fetch message
Android App-->>Android App: Send Message using Android SMS API
Android App-->>httpSMS API: [Async] Send result of sending SMS
Android App-->>httpSMS API: [Async] Send Delivery Report
```

## Self Host Setup - Docker

### 1. Setup SMTP Email service

The httpSMS application uses [SMTP](https://en.wikipedia.org/wiki/Simple_Mail_Transfer_Protocol) to send emails to users e.g. when your Android phone has been offline for a long period of time.
You can use a service like [mailtrap](https://mailtrap.io/) to create an SMTP server for development purposes.

### 2. Setup Cloudflare Turnstile

The message search route (`/v1/messages/search`) is protected by a [Cloudflare Turnstile](https://developers.cloudflare.com/turnstile/get-started/) captcha to prevent abuse. You need to set up a Turnstile widget for the search messages feature to work.

1. Go to the [Cloudflare dashboard](https://dash.cloudflare.com/) and navigate to **Turnstile**.
2. Add a new site and configure it for your self-hosted domain (e.g., `localhost` for local development).
3. Note down the **Site Key** and **Secret Key** — you will need them for the frontend and backend environment variables respectively.

### 3. Download the code

Clone the httpSMS GitHub repository

```bash
git clone https://github.com/NdoleStudio/httpsms.git
```

### 4. Setup the environment variables

- Copy the `.env.docker` file in the `web` directory into `.env`

```bash
cp web/.env.docker web/.env
```

- Update the environment variables in the `.env` file in the `web` directory (the SPA reads `API_BASE_URL` for the API endpoint)

```dotenv
# Cloudflare Turnstile site key from step 2
CLOUDFLARE_TURNSTILE_SITE_KEY=
```

- Copy the `.env.docker` file in the `api` directory into `.env`

```bash
cp api/.env.docker api/.env
```

- Update the environment variables in the `.env` file in the `api` directory with your SMTP server details and Cloudflare Turnstile secret key.

```dotenv
# SMTP email server settings
SMTP_USERNAME=
SMTP_PASSWORD=
SMTP_HOST=
SMTP_PORT=

# Cloudflare Turnstile secret key from step 2
CLOUDFLARE_TURNSTILE_SECRET_KEY=
```

- Don't bother about the `EVENTS_QUEUE_USER_API_KEY` and `EVENTS_QUEUE_USER_ID` settings. We will set that up later.

### 5. Build and Run

- Build and run the API, the web UI, database and cache using the `docker-compose.yml` file. It takes a while for build and download all the docker images.
  When it's finished, you'll be able to access the web UI at http://localhost:3000 and the API at http://localhost:8000

```bash
docker compose up --build
```

### 6. Create the System User

- The application uses the concept of a system user to process events async. You should manually create this user in `users` table in your database. Make sure you use the same `id` and `api_key` as the `EVENTS_QUEUE_USER_ID`, and `EVENTS_QUEUE_USER_API_KEY` in your `.env` file.

  ```SQL
  INSERT INTO users (id, api_key, email ) VALUES ('your-system-user-id', 'your-system-api-key', 'system@domain.com');
  ```

> [!IMPORTANT]
> Restart your API docker container after modifying `EVENTS_QUEUE_USER_ID`, and `EVENTS_QUEUE_USER_API_KEY` in your `.env` file so that the httpSMS API can pick up the changes.

### 7. Build the Android App.

- Open the `android` directory in [Android Studio](https://developer.android.com/studio) and build/run the app.
- No Google Play Services, Firebase or `google-services.json` is involved: the app long-polls your API for outbound messages through a foreground service, so it works on GMS-less devices (e.g. mainland China).

## Integration Testing

The project includes end-to-end integration tests that validate the complete SMS send/receive lifecycle. Tests run the full stack (API, PostgreSQL, Redis) in Docker alongside a phone emulator that simulates an Android device.

📖 **Full documentation:** [`tests/README.md`](tests/README.md)

**Quick run:**

```bash
cd tests
docker compose up -d --build --wait
docker compose wait seed && sleep 2
go test -v -timeout 120s ./...
docker compose down -v
```

Integration tests also run automatically in CI on every push/PR to `main`.

## 1&1 SMS-Flat — Usage Restrictions (Germany)

Um Missbrauch vorzubeugen, gelten für alle Tarife mit SMS-Flat als Tarifkomponente
oder als zubuchbare Option folgende zusätzliche Regelungen:

- Der automatisierte Versand von SMS ist unzulässig.
- Der Versand von SMS an mehr als 50 Empfänger gleichzeitig ist unzulässig.
- Die Nutzung der Zusatzdienstleistung mit einem SMS-Modemzugang und/oder mit SMS zu E-Mail
  oder Fax ist unzulässig.

1&1 geht weiterhin von einer missbräuchlichen Nutzung bei mehr als 3.000 SMS pro Monat aus.
In diesem Fall behält sich 1&1 vor, das Vertragsverhältnis außerordentlich zu kündigen.

## License

This project is licensed under the GNU AFFERO GENERAL PUBLIC LICENSE Version 3 - see the [LICENSE](LICENSE) file for details
