package main

import (
	"bufio"
	"encoding/json" // New: for JSON handling
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

// --- Global State ---

var (
	// Key format: "chatID:GAMENAME"
	subscriptions sync.Map 
	gameConfigs   sync.Map
	defaultGames  = []string{"DOTA2", "PUBG", "CUSTOM"}
	dbPath        = "/etc/tg-bot/subscriptions.json" // JSON path on your router
)

func main() {
	token := flag.String("t", os.Getenv("TG_TOKEN"), "Bot token")
	conf := flag.String("c", "/etc/tg-bot/game.conf", "Config path")
	flag.Parse()

	if *token == "" { log.Fatal("Token required.") }

	loadConfigs(*conf)
	loadSubscriptions() // Load saved users on start

	b, err := telebot.NewBot(telebot.Settings{
		Token:  *token,
		Poller: &telebot.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil { log.Fatal(err) }

	// --- Handlers ---

	b.Handle("/start", func(c telebot.Context) error {
		return c.Send("🎮 <b>BPI-R4 Multi-Group Bot Active</b>\nPersistent storage enabled.", telebot.ModeHTML)
	})

	b.Handle("/join", handleJoin)
	b.Handle("/goplay", handleGoPlay)
	b.Handle("/gamenow", handleGameNow)
	b.Handle("/list", handleList)
	b.Handle("/leave", handleLeave) // New: Allow users to unsubscribe

	b.Handle(telebot.OnQuery, handleInlineQuery)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	log.Printf("Starting bot @%s...", b.Me.Username)
	go b.Start()
	<-stop
	saveSubscriptions() // Save one last time on shutdown
	b.Stop()
}

// --- Persistence Logic ---

func saveSubscriptions() {
	data := make(map[string][]UserSubscription)
	subscriptions.Range(func(key, value interface{}) bool {
		data[key.(string)] = value.([]UserSubscription)
		return true
	})

	bytes, err := json.Marshal(data)
	if err != nil {
		log.Println("Error encoding JSON:", err)
		return
	}

	err = os.WriteFile(dbPath, bytes, 0644)
	if err != nil {
		log.Println("Error writing to storage:", err)
	}
}

func loadSubscriptions() {
	bytes, err := os.ReadFile(dbPath)
	if err != nil {
		log.Println("No existing subscription file found. Starting fresh.")
		return
	}

	var data map[string][]UserSubscription
	if err := json.Unmarshal(bytes, &data); err != nil {
		log.Println("Error decoding JSON:", err)
		return
	}

	for k, v := range data {
		subscriptions.Store(k, v)
	}
	log.Println("Successfully loaded persistent subscriptions.")
}

// --- Logic Helpers ---

func getSubKey(chatID int64, game string) string {
	return fmt.Sprintf("%d:%s", chatID, strings.ToUpper(game))
}

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

func sendGame(b *telebot.Bot, chatID int64, game, msg, from string) error {
	mentionStr := getMentions(chatID, game)
	if mentionStr == "" { mentionStr = "<i>No subscribers in this chat yet.</i>" }

	txt := fmt.Sprintf("🎮 <b>%s</b>\n%s\n\nInvited by: @%s\n\n🔔 <b>Attention:</b> %s", 
		html.EscapeString(game), html.EscapeString(msg), html.EscapeString(from), mentionStr)

	m, err := b.Send(telebot.ChatID(chatID), txt, telebot.ModeHTML)
	if err == nil { return b.Pin(m) }
	return err
}

// --- Handlers ---

func handleJoin(c telebot.Context) error {
	args := c.Args()
	if len(args) < 1 { return c.Send("❌ Usage: /join <game>") }
	
	game := strings.ToUpper(args[0])
	key := getSubKey(c.Chat().ID, game)
	user := UserSubscription{ID: c.Sender().ID, FirstName: c.Sender().FirstName}
	
	actual, _ := subscriptions.LoadOrStore(key, []UserSubscription{})
	list := actual.([]UserSubscription)
	
	for _, u := range list {
		if u.ID == user.ID { return c.Reply("✨ Already subscribed.") }
	}
	
	list = append(list, user)
	subscriptions.Store(key, list)
	saveSubscriptions() // Save to JSON instantly
	
	return c.Reply(fmt.Sprintf("✅ Subscribed to <b>%s</b>!", html.EscapeString(game)), telebot.ModeHTML)
}

func handleLeave(c telebot.Context) error {
	args := c.Args()
	if len(args) < 1 { return c.Send("❌ Usage: /leave <game>") }
	
	game := strings.ToUpper(args[0])
	key := getSubKey(c.Chat().ID, game)
	
	val, ok := subscriptions.Load(key)
	if !ok { return c.Reply("This category doesn't exist.") }
	
	list := val.([]UserSubscription)
	newList := []UserSubscription{}
	removed := false
	for _, u := range list {
		if u.ID != c.Sender().ID {
			newList = append(newList, u)
		} else {
			removed = true
		}
	}
	
	if removed {
		subscriptions.Store(key, newList)
		saveSubscriptions() // Save changes
		return c.Reply("🗑 Unsubscribed from " + game)
	}
	return c.Reply("You weren't in that list.")
}

func handleGoPlay(c telebot.Context) error {
	args := c.Args()
	if len(args) < 1 { return c.Send("❌ Usage: /goplay <game> [mins]") }
	game := strings.ToUpper(args[0])
	msg, delay := "Get ready!", 0
	if v, ok := gameConfigs.Load(game); ok {
		cfg := v.(GameConfig)
		msg, delay = cfg.Message, cfg.Time
	}
	if len(args) > 1 {
		if d, err := strconv.Atoi(args[1]); err == nil { delay = d }
	}

	if delay > 0 {
		mentions := getMentions(c.Chat().ID, game)
		confirmMsg := fmt.Sprintf("⏳ Scheduled: <b>%s</b> in %d mins.", html.EscapeString(game), delay)
		if mentions != "" { confirmMsg += "\n\n🔔 <b>Heads up:</b> " + mentions }

		time.AfterFunc(time.Duration(delay)*time.Minute, func() {
			_ = sendGame(c.Bot(), c.Chat().ID, game, msg, "Scheduled System")
		})
		return c.Reply(confirmMsg, telebot.ModeHTML)
	}
	return sendGame(c.Bot(), c.Chat().ID, game, msg, c.Sender().Username)
}

func handleList(c telebot.Context) error {
	var output strings.Builder
	output.WriteString("📋 <b>Group Categories:</b>\n")
	prefix := fmt.Sprintf("%d:", c.Chat().ID)
	found := false
	subscriptions.Range(func(key, value interface{}) bool {
		k := key.(string)
		if strings.HasPrefix(k, prefix) {
			found = true
			name := strings.TrimPrefix(k, prefix)
			list := value.([]UserSubscription)
			output.WriteString(fmt.Sprintf("\n🔹 <b>%s</b> (%d players)", html.EscapeString(name), len(list)))
		}
		return true
	})
	if !found { return c.Send("📋 No categories yet.") }
	return c.Send(output.String(), telebot.ModeHTML)
}

func handleGameNow(c telebot.Context) error {
	args := c.Args()
	if len(args) < 1 { return c.Send("❌ Usage: /gamenow <game>") }
	return sendGame(c.Bot(), c.Chat().ID, strings.ToUpper(args[0]), "Starting NOW!", c.Sender().Username)
}

func handleInlineQuery(c telebot.Context) error {
	results := telebot.Results{}
	for i, cat := range defaultGames {
		results = append(results, &telebot.ArticleResult{Title: cat, Text: "/gamenow " + cat})
		if i > 40 { break }
	}
	return c.Answer(&telebot.QueryResponse{Results: results, CacheTime: 60})
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
