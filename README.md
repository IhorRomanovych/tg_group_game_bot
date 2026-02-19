# 🎮 Telegram Game Bot for OpenWRT / BPI-R4

A high-performance, ultra-lightweight Telegram bot designed to run on the **Banana Pi R4** router. It allows group members to subscribe to specific game categories and receive targeted "Attention" pings when a session is starting.

## 🚀 Features

### Category-Based Subscriptions

* **Chat Isolation**: Subscriptions are specific to each group; joining a game in one group won't tag you in another.
* **`/join <game>`**: Users opt-in to specific categories (e.g., DOTA2, PUBG) to receive notifications.
* **`/leave <game>`**: Users can unsubscribe from any category at any time.
* **`/list`**: View all active game categories and subscriber counts for the current group.

### Game Announcements

* **`/gamenow <game>`**: Instantly pings all subscribers of a category and pins the invitation.
* **`/goplay <game> [mins]`**: Schedules a game. Tags subscribers **immediately** in the confirmation message and sends the final invitation after the delay.
* **Inline Suggestions**: Type `@your_bot_name ` to see a pop-up menu of available game categories.

### Technical Optimizations for BPI-R4

* **JSON Persistence**: Automatically saves subscriptions to `/etc/tg-bot/subscriptions.json`. Users don't need to re-join after a router reboot.
* **Ultra-Lightweight**: Built in Go; uses `sync.Map` and `time.AfterFunc` for near-zero idle CPU and minimal RAM footprint.
* **HTML Formatting**: Uses robust HTML parsing to prevent crashes from special characters or underscores in usernames.
* **Procd Integration**: Fully compatible with OpenWRT's process manager for auto-restart on crash and boot-start.

---

## 🛠 Installation

### 1. Build for BPI-R4 (ARM64)

Run this on your development machine to cross-compile for the router's architecture:

```bash
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o tg-bot main.go

```

*The flags `-s -w` strip debug symbols to reduce the binary size to ~5MB.*

### 2. Deploy to OpenWRT

1. **Upload the binary**:
```bash
scp tg-bot root@<router-ip>:/usr/bin/
chmod +x /usr/bin/tg-bot

```


2. **Create the config directory**:
```bash
mkdir -p /etc/tg-bot

```


3. **Configure the Service**: Create `/etc/init.d/tgbot` and paste your `procd` script. Then enable it:
```bash
/etc/init.d/tgbot enable
/etc/init.d/tgbot start

```



---

## ⚙️ Configuration (`game.conf`)

Store your game-specific messages in `/etc/tg-bot/game.conf`:

```ini
[DOTA2]
msg=Ranked match starting! ⚔️
time=5

[PUBG]
msg=Winner Winner Chicken Dinner! 🍗

```

## 🤖 BotFather Setup

To ensure the best user experience, configure your bot with these commands:

1. **Set Command List**:
```text
join - Subscribe to a game category
leave - Unsubscribe from a category
list - Show active categories in this group
goplay - Schedule a game (tags everyone now)
gamenow - Start a game immediately
statusreset - Clear your personal data

```


2. **Enable Inline Mode**: Send `/setinline` to `@BotFather` to enable the category suggestion menu.
