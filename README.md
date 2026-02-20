# 🎮 Telegram Game Bot for OpenWRT / BPI-R4

A high-performance, ultra-lightweight Telegram bot designed to run on the **Banana Pi R4** router. It allows group members to subscribe to specific game categories and receive targeted "Attention" pings when a session is starting.

## 🚀 Features

### Category-Based Subscriptions

* **Chat Isolation**: Subscriptions are scoped to each group via `ChatID`; joining a game in one group won't tag you in another.
* **`/join <game>`**: Opt-in to specific categories (e.g., DOTA2, CS2) to receive notifications.
* **`/leave <game>`**: Unsubscribe from a category.
* **`/list`**: A silent command that lists all active categories in the current group and shows who is subscribed to each.

### Game Announcements

* **`/gamenow <game>`**: Instantly pings all subscribers (up to 50 per category) and **pins** the invitation message.
* **`/goplay <game> [mins]`**: Schedules a game. If a time is provided, it triggers a timer; otherwise, it pings immediately. All invitations automatically attempt to pin the message for maximum visibility.

### 🛡️ Admin & Moderation

The bot includes a robust management system for the owner (set via the `-o` flag):

* **`/ban <user_id>`**: Restricts a specific user from using the bot.
* **`/unban <user_id>`**: Lifts restrictions on a user.
* **`/banlist`**: Displays all currently restricted Telegram IDs.
* **`/rmcat <GAME>`**: Force-deletes a game category and all its subscriptions from the group.

### Technical Optimizations for BPI-R4

* **Atomic Persistence**: Automatically saves subscriptions and the ban list to `/etc/tg-bot/subscriptions.json`.
* **Graceful Shutdown**: Handles `SIGINT` and `SIGTERM` to ensure all data is flushed to disk before the process exits.
* **Resource Efficient**: Built in Go using `sync.Map` for thread-safe operations without heavy locking, ensuring near-zero CPU impact on your router.

---

## 🛠 Installation & Deployment

### 1. Build for BPI-R4 (ARM64)

Run this on your development machine to cross-compile for the router's architecture:

```bash
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o tg-bot main.go

```

### 2. Set Up the OpenWRT Service

Create a new file at `/etc/init.d/tgbot` on your router and paste the following `procd` script:

```sh
#!/bin/sh /etc/rc.common

START=99
USE_PROCD=1

PROG="/usr/bin/tg-bot"
CONFIG="/etc/tg-bot/game.conf"
TOKEN="BOT_TOKEN"
OWNER=OWNER_ID

start_service() {
    procd_open_instance
    # Pass owner ID flag to ensure admin rights are active
    procd_set_param command "$PROG" -t "$TOKEN" -c "$CONFIG" -o "$OWNER"

    # Respawn logic: 5 attempts in 1 hour max, 5s delay between restarts
    procd_set_param respawn 3600 5 5

    procd_set_param stdout 1
    procd_set_param stderr 1
    procd_close_instance
}

stop_service() {
    # procd handles SIGTERM automatically, triggering saveData() in the Go code
    return 0
}

```

### 3. Activate the Bot

```bash
chmod +x /usr/bin/tg-bot
chmod +x /etc/init.d/tgbot
/etc/init.d/tgbot enable
/etc/init.d/tgbot start

```

---

## ⚙️ Configuration (`game.conf`)

Store custom messages in `/etc/tg-bot/game.conf`:

```ini
[DOTA2]
msg=Ranked match starting! ⚔️
time=5

[PUBG]
msg=Winner Winner Chicken Dinner! 🍗

```

---

## 🤖 BotFather Setup

Configure your bot with these commands for the best user experience:

```text
join - Subscribe to a game category
leave - Unsubscribe from a category
list - Show active categories and users
goplay - Schedule a game (pings subscribers)
gamenow - Start a game immediately
ban - (Admin) Restrict a user ID
unban - (Admin) Remove restriction
banlist - (Admin) Show restricted IDs
rmcat - (Admin) Delete a category

```