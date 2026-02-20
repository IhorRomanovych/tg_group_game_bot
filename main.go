package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"gopkg.in/telebot.v3"
)

// --- Models ---

type GameConfig struct {
	Message string
	Time    int
}

type UserSubscription struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
}

type BotStorage struct {
	Subscriptions map[string][]UserSubscription `json:"subscriptions"`
	BanList       map[int64]bool                `json:"ban_list"`
}

// --- Global State ---

var (
	subscriptions sync.Map 
	banList       sync.Map 
	gameConfigs   sync.Map
	defaultGames  = []string{"DOTA2", "PUBG", "CUSTOM"}
	dbPath        = "/etc/tg-bot/subscriptions.json"
	ownerID       int64 = 0
)

func main() {
	token := flag.String("t", os.Getenv("TG_TOKEN"), "Bot token")
	conf := flag.String("c", "/etc/tg-bot/game.conf", "Config path")
	owner := flag.Int64("o", 0, "Owner Telegram ID")
	flag.Parse()

	if *token == "" { log.Fatal("Token required.") }
	ownerID = *owner

	loadConfigs(*conf)
	loadData()

	b, err := telebot.NewBot(telebot.Settings{
		Token:  *token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil { log.Fatal(err) }

	isAdmin := func(id int64) bool { return id == ownerID }

	// --- Handlers ---
	b.Handle("/start", func(c telebot.Context) error {
		return c.Send("🎮 <b>BPI-R4 Bot Active</b>\nGatherings tag everyone; /list is silent.", telebot.ModeHTML)
	})

	b.Handle("/join", handleJoin)
	b.Handle("/leave", handleLeave)
	b.Handle("/goplay", handleGoPlay)
	b.Handle("/gamenow", handleGameNow)
	b.Handle("/list", handleList)

	// Admin
	b.Handle("/ban", func(c telebot.Context) error {
		if !isAdmin(c.Sender().ID) { return nil }
		id, _ := strconv.ParseInt(c.Args()[0], 10, 64)
		banList.Store(id, true)
		saveData()
		return c.Send(fmt.Sprintf("🔨 User %d restricted (No tags in /list).", id))
	})

	b.Handle("/unban", func(c telebot.Context) error {
		if !isAdmin(c.Sender().ID) { return nil }
		id, _ := strconv.ParseInt(c.Args()[0], 10, 64)
		banList.Delete(id)
		saveData()
		return c.Send(fmt.Sprintf("✅ User %d unrestricted.", id))
	})

	b.Handle("/rmcat", func(c telebot.Context) error {
		if !isAdmin(c.Sender().ID) { return nil }
		game := strings.ToUpper(c.Args()[0])
		subscriptions.Delete(getSubKey(c.Chat().ID, game))
		saveData()
		return c.Send("🗑 Category removed.")
	})

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go b.Start()
	<-stop
	saveData()
	b.Stop()
}

// --- Logic Helpers ---

func getSubKey(chatID int64, game string) string {
	return fmt.Sprintf("%d:%s", chatID, strings.ToUpper(game))
}

// Tags EVERYONE when a gathering starts (per your requirement)
func getMentions(chatID int64, game string) string {
	var mentions []string
	key := getSubKey(chatID, game)
	if val, ok := subscriptions.Load(key); ok {
		list := val.([]UserSubscription)
		for i, u := range list {
			if i >= 50 { break }
			mentions = append(mentions, fmt.Sprintf("<a href=\"tg://user?id=%d\">%s</a>", u.ID, html.EscapeString(u.FirstName)))
		}
	}
	return strings.Join(mentions, ", ")
}

// --- Handlers ---

func handleList(c telebot.Context) error {
	var output strings.Builder
	output.WriteString("📋 <b>Group Categories:</b>\n")
	prefix := fmt.Sprintf("%d:", c.Chat().ID)
	found := false

	subscriptions.Range(func(key, value interface{}) bool {
		k := key.(string)
		if strings.HasPrefix(k, prefix) {
			found = true
			gameName := strings.TrimPrefix(k, prefix)
			list := value.([]UserSubscription)
			output.WriteString(fmt.Sprintf("\n🔹 <b>%s</b> (%d): ", gameName, len(list)))

			var names []string
			for _, u := range list {
				// CHANGE: Everyone is just plain text here. NO TAGS.
				names = append(names, html.EscapeString(u.FirstName))
			}
			output.WriteString(strings.Join(names, ", "))
		}
		return true
	})
	if !found { return c.Send("📋 No active categories.") }
	return c.Send(output.String(), telebot.ModeHTML)
}

func handleJoin(c telebot.Context) error {
	if len(c.Args()) < 1 { return c.Send("❌ Usage: /join <game>") }
	game := strings.ToUpper(c.Args()[0])
	key := getSubKey(c.Chat().ID, game)
	user := UserSubscription{ID: c.Sender().ID, FirstName: c.Sender().FirstName}

	actual, _ := subscriptions.LoadOrStore(key, []UserSubscription{})
	list := actual.([]UserSubscription)
	for _, u := range list {
		if u.ID == user.ID { return c.Reply("✨ Already in list.") }
	}
	list = append(list, user)
	subscriptions.Store(key, list)
	saveData()
	return c.Reply("✅ Joined " + game)
}

func sendGame(b *telebot.Bot, chatID int64, game, msg, from string) error {
	mentionStr := getMentions(chatID, game)
	txt := fmt.Sprintf("🎮 <b>%s</b>\n%s\n\nInvited by: %s\n\n🔔 %s", 
		html.EscapeString(game), html.EscapeString(msg), html.EscapeString(from), mentionStr)
	
	m, err := b.Send(telebot.ChatID(chatID), txt, telebot.ModeHTML)
	if err == nil { _ = b.Pin(m) }
	return err
}

func handleGoPlay(c telebot.Context) error {
	if len(c.Args()) < 1 { return c.Send("❌ Usage: /goplay <game>") }
	game := strings.ToUpper(c.Args()[0])
	msg, delay := "Get ready!", 0
	if v, ok := gameConfigs.Load(game); ok {
		cfg := v.(GameConfig)
		msg, delay = cfg.Message, cfg.Time
	}
	if len(c.Args()) > 1 {
		if d, err := strconv.Atoi(c.Args()[1]); err == nil { delay = d }
	}
	if delay > 0 {
		time.AfterFunc(time.Duration(delay)*time.Minute, func() {
			_ = sendGame(c.Bot(), c.Chat().ID, game, msg, "Scheduled System")
		})
		return c.Reply(fmt.Sprintf("⏳ Scheduled %s in %d mins.", game, delay))
	}
	return sendGame(c.Bot(), c.Chat().ID, game, msg, c.Sender().Username)
}

func handleLeave(c telebot.Context) error {
	if len(c.Args()) < 1 { return c.Send("❌ Usage: /leave <game>") }
	game := strings.ToUpper(c.Args()[0])
	key := getSubKey(c.Chat().ID, game)
	if val, ok := subscriptions.Load(key); ok {
		list := val.([]UserSubscription)
		var newList []UserSubscription
		for _, u := range list {
			if u.ID != c.Sender().ID { newList = append(newList, u) }
		}
		subscriptions.Store(key, newList)
		saveData()
	}
	return c.Reply("🗑 Left " + game)
}

func handleGameNow(c telebot.Context) error {
	if len(c.Args()) < 1 { return c.Send("❌ /gamenow <game>") }
	return sendGame(c.Bot(), c.Chat().ID, strings.ToUpper(c.Args()[0]), "Starting NOW!", c.Sender().Username)
}

// --- Persistence ---

func saveData() {
	store := BotStorage{
		Subscriptions: make(map[string][]UserSubscription),
		BanList:       make(map[int64]bool),
	}
	subscriptions.Range(func(k, v any) bool {
		store.Subscriptions[k.(string)] = v.([]UserSubscription)
		return true
	})
	banList.Range(func(k, v any) bool {
		store.BanList[k.(int64)] = true
		return true
	})
	bytes, _ := json.Marshal(store)
	_ = os.WriteFile(dbPath, bytes, 0644)
}

func loadData() {
	bytes, err := os.ReadFile(dbPath)
	if err != nil { return }
	var store BotStorage
	if err := json.Unmarshal(bytes, &store); err == nil {
		for k, v := range store.Subscriptions { subscriptions.Store(k, v) }
		for k, v := range store.BanList { banList.Store(k, v) }
		return
	}
	var oldData map[string][]UserSubscription
	if err := json.Unmarshal(bytes, &oldData); err == nil {
		for k, v := range oldData { subscriptions.Store(k, v) }
	}
}

func loadConfigs(p string) {
	f, err := os.Open(p)
	if err != nil { return }
	defer f.Close()
	s := bufio.NewScanner(f)
	var cur string
	for s.Scan() {
		ln := strings.TrimSpace(s.Text())
		if strings.HasPrefix(ln, "[") {
			cur = strings.ToUpper(strings.Trim(ln, "[]"))
			gameConfigs.Store(cur, GameConfig{Message: "Game on!"})
		} else if cur != "" && strings.Contains(ln, "=") {
			p := strings.SplitN(ln, "=", 2)
			if val, ok := gameConfigs.Load(cur); ok {
				cfg := val.(GameConfig)
				k, v := strings.TrimSpace(p[0]), strings.TrimSpace(p[1])
				if k == "msg" { cfg.Message = v }
				if k == "time" { cfg.Time, _ = strconv.Atoi(v) }
				gameConfigs.Store(cur, cfg)
			}
		}
	}
}