# Check-In Bot

A Telegram bot that facilitates a check-in system. Registered users receive pings and respond to confirm they're okay. Users can silence notifications, send messages to a shared admin inbox, and admins get alerted when someone misses a check-in. Includes a web UI for administration, authenticated via Telegram Login Widget.

Ships as a single Go binary with the frontend embedded.

## Features

- **Check-in pings** — bot messages users with check-ins enabled on a configurable UTC cron schedule
- **Per-user schedule overrides** — admins can optionally give individual users their own cron schedule in the web UI
- **One-tap check-in** — users respond via inline buttons ("Check In" or "Check In + Note")
- **Silence notifications** — users can mute pings for 1–7 days with an optional reason
- **Missed check-in alerts** — if a user still has an unanswered check-in by the time their next scheduled check-in would occur, all admins get a Telegram message
- **Shared admin inbox** — registered users can send freeform messages or use the in-chat menu to reach admins
- **Admin web UI** — dashboard, split admin/user management, internal user notes, inbox, and silence management
- **Telegram Login** — web UI authenticated via the Telegram Login Widget (no passwords)
- **Single binary** — frontend is built with Bun/Preact and embedded into the Go binary

## Requirements

- Go 1.26.1+
- [Bun](https://bun.sh/) (for building the frontend)
- A Telegram bot token from [@BotFather](https://t.me/botfather)

## Quick Start

1. **Create a bot** with [@BotFather](https://t.me/botfather) and note the token.

2. **Register your domain** with BotFather for the Telegram Login Widget:
   ```
   /setdomain
   ```
   Set it to the domain where the web UI will be accessible (e.g. `checkin.example.com`).

3. **Build**:
   ```sh
   make build
   ```

4. **Run**:
   ```sh
     ./check-in-bot \
     --token "123456:ABC-DEF..." \
     --base-url "https://checkin.example.com" \
     --admin-id "12345678"
   ```

   Or use environment variables:
   ```sh
   export CHECKIN_TOKEN="123456:ABC-DEF..."
   export CHECKIN_BASE_URL="https://checkin.example.com"
   export CHECKIN_ADMIN_ID="12345678"
   ./check-in-bot
   ```

5. **Local testing** (no domain needed):
   ```sh
   ./check-in-bot \
     --token "123456:ABC-DEF..." \
     --admin-id "YOUR_TELEGRAM_ID" \
     --dev
   ```
   Dev mode bypasses the Telegram Login Widget. Visit `http://localhost:8080` and click "Dev Login" to auto-authenticate as the first admin user. The bot still works normally via Telegram — only the web UI login is bypassed.

## Helm

A minimal Helm chart is available at `charts/check-in-bot`.

Example install:

```sh
helm upgrade --install check-in-bot ./charts/check-in-bot \
  --set image.repository=ghcr.io/markpash/check-in-bot \
  --set image.tag=latest \
  --set secret.telegramToken="123456:ABC-DEF..." \
  --set config.baseURL="https://checkin.example.com" \
  --set config.adminId="12345678"
```

Important values:

- `secret.telegramToken` — the Telegram bot token
- `config.baseURL` — the public URL used by the web UI and Telegram login flow
- `config.adminId` — the initial seed admin Telegram numeric ID
- `config.checkinSchedule` — the global UTC cron schedule for check-ins
- `persistence.enabled` — keeps the SQLite database on a persistent volume
- `ingress.enabled` — exposes the web UI through Kubernetes ingress

## Configuration

All flags can also be set via environment variables with the `CHECKIN_` prefix.

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--token` | `CHECKIN_TOKEN` | *(required)* | Telegram bot token |
| `--base-url` | `CHECKIN_BASE_URL` | `http://localhost:8080` | Public URL of the web UI (must match BotFather domain) |
| `--db` | `CHECKIN_DB` | `checkin.db` | SQLite database file path |
| `--listen` | `CHECKIN_LISTEN` | `:8080` | HTTP listen address |
| `--checkin-schedule` | `CHECKIN_CHECKIN_SCHEDULE` | `0 9 * * *` | UTC cron schedule for check-in pings (minute hour day-of-month month day-of-week) |
| `--admin-id` | `CHECKIN_ADMIN_ID` | | Telegram user ID to seed as the initial admin |
| `--dev` | `CHECKIN_DEV` | `false` | Enable dev mode (bypasses Telegram Login for web UI) |

## Telegram UX

The bot is primarily button-driven inside Telegram.

### Public command

| Command | Description |
|---------|-------------|
| `/start <invite_code>` | Register with the bot using an invite code |

If a user was already added manually by Telegram ID in the web UI, they can also just press `Start` without an invite code.

### Registered-user menu

After registration, the bot shows a persistent reply-keyboard menu with:

- `Status` — show the latest check-in and silence status
- `Silence` — choose a 1-7 day silence period, with an optional reason
- `Unsilence` — cancel an active silence early
- `Message Admins` — send a message to the shared admin inbox
- `Help` — show the menu explanation again

Any freeform text sent to the bot outside the check-in-note flow is also routed to the admin inbox.

### Compatibility slash commands

The button menu is the intended UX, but these registered-user slash commands still work:

| Command | Description |
|---------|-------------|
| `/help` | Show the menu/help text |
| `/status` | Show your last check-in and silence status |
| `/silence N [reason]` | Silence pings for N days (1-7), with optional reason |
| `/unsilence` | Cancel an active silence early |
| `/msg <text>` | Send a message to the admin inbox |

There are no admin bot commands. Admin actions happen only through the web UI.

## Web UI

The admin panel is served at the configured `--listen` address. It requires authentication via the Telegram Login Widget — only users marked as admins in the database can log in.

### Pages

- **Dashboard** — today's check-in status at a glance: missed (red), pending (yellow), checked in (green), and silenced (gray)
- **Users** — manage users, invite onboarding, nicknames, notes, check-in enablement, and per-user cron schedules
- **Inbox** — shared message inbox from users, with read/unread filtering and bulk mark-as-read
- **Silences** — view and cancel active silences

## How It Works

### Check-in flow

1. On each configured cron tick, the bot sends a message to every user with check-ins enabled and no active silence, with two inline buttons: **Check In** and **Check In + Note**.
2. Pressing **Check In** immediately records the check-in.
3. Pressing **Check In + Note** prompts the user to type a note, then records the check-in with that note attached.
4. A user can follow the global cron schedule or their own per-user cron override if one is set in the web UI.
5. If a user still has an unanswered check-in when their next scheduled check-in time passes, that earlier check-in is considered missed and all admins receive a Telegram alert.

### Duplicate prevention

- The pinger tracks each scheduled run window so restarting the bot won't resend the same scheduled check-in.
- The missed check-in monitor marks alerts as sent so admins aren't notified repeatedly for the same missed check-in.

### Silencing

When a user silences notifications, they are excluded from both the pinger and the missed check-in alerts for the silence period. If they silence after receiving a ping, the pending check-in is still visible on the dashboard but won't trigger an admin alert. Users with check-ins disabled are also excluded from missed check-in alerts.


## Database

Uses SQLite with WAL mode. The schema is created automatically on first run. Tables:

- **users** — Telegram user ID as primary key, admin and check-in settings
- **checkins** — Ping records with response timestamps
- **silences** — Active silence periods with start/end times
- **messages** — Shared admin inbox
- **sessions** — Web UI login sessions
