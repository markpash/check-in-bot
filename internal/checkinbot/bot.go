package checkinbot

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	tele "gopkg.in/telebot.v4"
)

var (
	checkInMarkup  = &tele.ReplyMarkup{}
	btnCheckIn     = checkInMarkup.Data("Check In", "checkin")
	btnCheckInNote = checkInMarkup.Data("Check In + Note", "checkin_note")

	mainMenuMarkup    = &tele.ReplyMarkup{ResizeKeyboard: true}
	silenceMenuMarkup = &tele.ReplyMarkup{ResizeKeyboard: true, OneTimeKeyboard: true}
	reasonMenuMarkup  = &tele.ReplyMarkup{ResizeKeyboard: true, OneTimeKeyboard: true}

	btnMenuStatus    = mainMenuMarkup.Text("Status")
	btnMenuSilence   = mainMenuMarkup.Text("Silence")
	btnMenuUnsilence = mainMenuMarkup.Text("Unsilence")
	btnMenuMessage   = mainMenuMarkup.Text("Message Admins")
	btnMenuHelp      = mainMenuMarkup.Text("Help")

	btnSilence1Day = silenceMenuMarkup.Text("1 day")
	btnSilence2Day = silenceMenuMarkup.Text("2 days")
	btnSilence3Day = silenceMenuMarkup.Text("3 days")
	btnSilence4Day = silenceMenuMarkup.Text("4 days")
	btnSilence5Day = silenceMenuMarkup.Text("5 days")
	btnSilence6Day = silenceMenuMarkup.Text("6 days")
	btnSilence7Day = silenceMenuMarkup.Text("7 days")

	btnSkip   = reasonMenuMarkup.Text("Skip")
	btnCancel = reasonMenuMarkup.Text("Cancel")
)

type pendingAction struct {
	Kind      string
	Days      int
	CreatedAt time.Time
}

var pendingActions sync.Map // map[int64]pendingAction

const (
	actionComposeMessage = "compose_message"
	actionSilenceReason  = "silence_reason"
)

func init() {
	checkInMarkup.Inline(
		checkInMarkup.Row(btnCheckIn, btnCheckInNote),
	)

	mainMenuMarkup.Reply(
		mainMenuMarkup.Row(btnMenuStatus, btnMenuSilence),
		mainMenuMarkup.Row(btnMenuUnsilence, btnMenuMessage),
		mainMenuMarkup.Row(btnMenuHelp),
	)

	silenceMenuMarkup.Reply(
		silenceMenuMarkup.Row(btnSilence1Day, btnSilence2Day, btnSilence3Day),
		silenceMenuMarkup.Row(btnSilence4Day, btnSilence5Day, btnSilence6Day),
		silenceMenuMarkup.Row(btnSilence7Day, btnCancel),
	)

	reasonMenuMarkup.Reply(
		reasonMenuMarkup.Row(btnSkip, btnCancel),
	)

	go func() {
		for range time.Tick(5 * time.Minute) {
			now := time.Now()
			pendingActions.Range(func(key, value any) bool {
				action := value.(pendingAction)
				if now.Sub(action.CreatedAt) > 10*time.Minute {
					pendingActions.Delete(key)
				}
				return true
			})
		}
	}()
}

func SetupBot(cfg Config, db *sqlx.DB) (*tele.Bot, error) {
	b, err := tele.NewBot(tele.Settings{
		Token:  cfg.TelegramToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return nil, err
	}

	b.Use(privateChatOnlyMiddleware())

	// --- Public commands (no auth) ---
	b.Handle("/start", handleStart(db))

	// --- Registered user commands ---
	registeredUsers := b.Group()
	registeredUsers.Use(registeredUserMiddleware(db))
	registeredUsers.Handle("/help", handleHelp())
	registeredUsers.Handle("/msg", handleMsg(db))
	registeredUsers.Handle("/status", handleStatus(db))
	registeredUsers.Handle("/silence", handleSilence(db))
	registeredUsers.Handle("/unsilence", handleUnsilence(db))

	// --- Callbacks ---
	b.Handle(&btnCheckIn, handleCheckInCallback(db))
	b.Handle(&btnCheckInNote, handleCheckInNoteCallback(db))

	// --- Freeform text ---
	b.Handle(tele.OnText, handleText(db))

	return b, nil
}

// --- Middleware ---

func privateChatOnlyMiddleware() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			chat := c.Chat()
			if chat != nil && chat.Type != tele.ChatPrivate {
				return nil
			}
			return next(c)
		}
	}
}

func mainMenuOptions() *tele.SendOptions {
	return &tele.SendOptions{ReplyMarkup: mainMenuMarkup}
}

func silenceMenuOptions() *tele.SendOptions {
	return &tele.SendOptions{ReplyMarkup: silenceMenuMarkup}
}

func reasonMenuOptions() *tele.SendOptions {
	return &tele.SendOptions{ReplyMarkup: reasonMenuMarkup}
}

func sendMainMenu(c tele.Context, text string) error {
	return c.Send(text, mainMenuOptions())
}

func parseSilenceDaysSelection(text string) int {
	switch text {
	case btnSilence1Day.Text:
		return 1
	case btnSilence2Day.Text:
		return 2
	case btnSilence3Day.Text:
		return 3
	case btnSilence4Day.Text:
		return 4
	case btnSilence5Day.Text:
		return 5
	case btnSilence6Day.Text:
		return 6
	case btnSilence7Day.Text:
		return 7
	default:
		return 0
	}
}

func createUserSilence(db *sqlx.DB, userID int64, days int, reason string) error {
	cancelActiveSilences(db, userID)
	return createSilence(db, userID, days, reason)
}

func registeredUserMiddleware(db *sqlx.DB) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			s := c.Sender()
			_, err := getUser(db, s.ID)
			if err == sql.ErrNoRows {
				return c.Send("You need to register first. Use /start <invite_code>.")
			}
			if err != nil {
				log.Printf("registeredUserMiddleware: %v", err)
				return c.Send("An error occurred. Please try again.")
			}
			upsertUser(db, s.ID, s.Username, s.FirstName, s.LastName, s.LanguageCode, s.IsBot, s.IsPremium)
			return next(c)
		}
	}
}

// --- Public handlers ---

func handleStart(db *sqlx.DB) tele.HandlerFunc {
	return func(c tele.Context) error {
		s := c.Sender()
		code := strings.TrimSpace(c.Message().Payload)
		_, err := getUser(db, s.ID)
		if err == nil {
			upsertUser(db, s.ID, s.Username, s.FirstName, s.LastName, s.LanguageCode, s.IsBot, s.IsPremium)
			return sendMainMenu(c, "You are already registered.")
		}
		if err != sql.ErrNoRows {
			log.Printf("handleStart getUser: %v", err)
			return c.Send("An error occurred. Please try again.")
		}

		if code == "" {
			return c.Send("You need an invite code to register. Ask an admin for an invite link or use /start <invite_code>.")
		}

		invite, err := getInviteCode(db, code)
		if err == sql.ErrNoRows || invite.UsedBy != nil {
			return c.Send("That invite code is invalid or has already been used.")
		}
		if err != nil {
			log.Printf("getInviteCode: %v", err)
			return c.Send("An error occurred while checking your invite code.")
		}

		upsertUser(db, s.ID, s.Username, s.FirstName, s.LastName, s.LanguageCode, s.IsBot, s.IsPremium)
		ok, err := redeemInviteCode(db, code, s.ID)
		if err == sql.ErrNoRows || !ok {
			return c.Send("That invite code is invalid or has already been used.")
		}
		if err != nil {
			log.Printf("redeemInviteCode: %v", err)
			return c.Send("An error occurred while checking your invite code.")
		}

		if err := setUserCheckinsEnabled(db, s.ID, true); err != nil {
			log.Printf("setUserCheckinsEnabled: %v", err)
			return c.Send("Your account was created, but enabling check-ins failed. Please contact an admin.")
		}
		return sendMainMenu(c, "Welcome! Your invite code worked and your account has been registered.")
	}
}

func handleHelp() tele.HandlerFunc {
	return func(c tele.Context) error {
		helpText := strings.Join([]string{
			"Use the buttons below to interact with the bot:",
			"",
			"Status: see your latest check-in state and silence status.",
			"Silence: choose how many days to pause check-ins.",
			"Unsilence: cancel an active silence early.",
			"Message Admins: send a message to the admin inbox.",
			"Help: show this menu again.",
		}, "\n")
		return sendMainMenu(c, helpText)
	}
}

// --- Active user handlers ---

func handleStatus(db *sqlx.DB) tele.HandlerFunc {
	return func(c tele.Context) error {
		uid := c.Sender().ID

		// Last check-in
		var lastCheckin CheckIn
		err := db.Get(&lastCheckin, `
			SELECT * FROM checkins WHERE user_id = ? ORDER BY pinged_at DESC LIMIT 1
		`, uid)

		var statusMsg strings.Builder
		if err == sql.ErrNoRows {
			statusMsg.WriteString("No check-ins yet.\n")
		} else if err != nil {
			return c.Send("Error fetching status.")
		} else {
			statusMsg.WriteString(fmt.Sprintf("Last pinged: %s\n", lastCheckin.PingedAt))
			if lastCheckin.CheckedInAt != nil {
				statusMsg.WriteString(fmt.Sprintf("Checked in: %s\n", *lastCheckin.CheckedInAt))
			} else {
				statusMsg.WriteString("Status: Not yet checked in\n")
			}
		}

		// Silence status
		silenced, _ := isUserSilenced(db, uid)
		if silenced {
			statusMsg.WriteString("\nYou currently have an active silence.")
		} else {
			statusMsg.WriteString("\nNo active silence.")
		}

		return sendMainMenu(c, statusMsg.String())
	}
}

func handleSilence(db *sqlx.DB) tele.HandlerFunc {
	return func(c tele.Context) error {
		args := c.Args()
		if len(args) == 0 {
			return c.Send("Choose a silence duration from the menu.", silenceMenuOptions())
		}

		days, err := strconv.Atoi(args[0])
		if err != nil || days < 1 || days > 7 {
			return c.Send("Please provide a number of days between 1 and 7.")
		}

		reason := ""
		if len(args) > 1 {
			reason = strings.Join(args[1:], " ")
		}

		// Cancel any existing silence first
		if err := createUserSilence(db, c.Sender().ID, days, reason); err != nil {
			log.Printf("silence: %v", err)
			return c.Send("Failed to create silence.")
		}

		msg := fmt.Sprintf("Pings silenced for %d day(s).", days)
		if reason != "" {
			msg += fmt.Sprintf(" Reason: %s", reason)
		}
		return sendMainMenu(c, msg)
	}
}

func handleUnsilence(db *sqlx.DB) tele.HandlerFunc {
	return func(c tele.Context) error {
		affected, err := cancelActiveSilences(db, c.Sender().ID)
		if err != nil {
			log.Printf("unsilence: %v", err)
			return c.Send("Failed to cancel silence.")
		}
		if affected == 0 {
			return c.Send("You don't have an active silence.")
		}
		return sendMainMenu(c, "Silence cancelled. You'll receive pings again.")
	}
}

func handleMsg(db *sqlx.DB) tele.HandlerFunc {
	return func(c tele.Context) error {
		text := strings.TrimSpace(c.Message().Payload)
		if text == "" {
			pendingActions.Store(c.Sender().ID, pendingAction{
				Kind:      actionComposeMessage,
				CreatedAt: time.Now(),
			})
			return c.Send("Send the message you want to forward to the admins, or tap Cancel.", reasonMenuOptions())
		}
		return saveInboxMessage(db, c.Sender().ID, text, c)
	}
}

func saveInboxMessage(db *sqlx.DB, userID int64, body string, c tele.Context) error {
	s := c.Sender()
	upsertUser(db, s.ID, s.Username, s.FirstName, s.LastName, s.LanguageCode, s.IsBot, s.IsPremium)
	_, err := createMessage(db, userID, body)
	if err != nil {
		log.Printf("createMessage: %v", err)
		return sendMainMenu(c, "Failed to send message.")
	}

	admins, err := getAdminUsers(db)
	if err != nil {
		log.Printf("getAdminUsers: %v", err)
		return sendMainMenu(c, "Message sent to admins.")
	}

	senderName := strings.TrimSpace(strings.Join([]string{s.FirstName, s.LastName}, " "))
	if senderName == "" {
		senderName = "Unknown"
	}
	if s.Username != "" {
		senderName += fmt.Sprintf(" (@%s)", s.Username)
	}

	adminMsg := fmt.Sprintf("New inbox message from %s [ID: %d]\n\n%s", senderName, s.ID, body)
	for _, admin := range admins {
		if _, err := c.Bot().Send(admin, adminMsg); err != nil {
			log.Printf("notify admin %d: %v", admin.ID, err)
		}
	}

	return sendMainMenu(c, "Message sent to admins.")
}
