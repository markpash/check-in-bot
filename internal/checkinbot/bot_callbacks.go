package checkinbot

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	tele "gopkg.in/telebot.v4"
)

type pendingNote struct {
	checkinID int64
	msgID     int
	chatID    int64
	createdAt time.Time
}

var pendingNotes sync.Map // map[int64]pendingNote (keyed by Telegram user ID)

func init() {
	// Clean stale pending notes every 5 minutes.
	go func() {
		for range time.Tick(5 * time.Minute) {
			now := time.Now()
			pendingNotes.Range(func(key, value any) bool {
				pn := value.(pendingNote)
				if now.Sub(pn.createdAt) > 10*time.Minute {
					pendingNotes.Delete(key)
				}
				return true
			})
		}
	}()
}

func handleCheckInCallback(db *sqlx.DB) tele.HandlerFunc {
	return func(c tele.Context) error {
		msg := c.Callback().Message
		ci, err := getCheckinByMsg(db, msg.ID, msg.Chat.ID)
		if err == sql.ErrNoRows {
			return c.Respond(&tele.CallbackResponse{Text: "Check-in not found."})
		}
		if err != nil {
			log.Printf("handleCheckIn: %v", err)
			return c.Respond(&tele.CallbackResponse{Text: "Error."})
		}

		// Already checked in — just acknowledge.
		if ci.CheckedInAt != nil {
			return c.Respond(&tele.CallbackResponse{Text: "Already checked in!"})
		}

		if err := completeCheckin(db, ci.ID, ""); err != nil {
			log.Printf("completeCheckin: %v", err)
			return c.Respond(&tele.CallbackResponse{Text: "Error."})
		}

		now := time.Now().UTC().Format("15:04 UTC")
		c.Bot().Edit(msg, fmt.Sprintf("Checked in at %s.", now))
		return c.Respond(&tele.CallbackResponse{Text: "Checked in!"})
	}
}

func handleCheckInNoteCallback(db *sqlx.DB) tele.HandlerFunc {
	return func(c tele.Context) error {
		msg := c.Callback().Message
		ci, err := getCheckinByMsg(db, msg.ID, msg.Chat.ID)
		if err == sql.ErrNoRows {
			return c.Respond(&tele.CallbackResponse{Text: "Check-in not found."})
		}
		if err != nil {
			log.Printf("handleCheckInNote: %v", err)
			return c.Respond(&tele.CallbackResponse{Text: "Error."})
		}

		if ci.CheckedInAt != nil {
			return c.Respond(&tele.CallbackResponse{Text: "Already checked in!"})
		}

		pendingNotes.Store(c.Sender().ID, pendingNote{
			checkinID: ci.ID,
			msgID:     msg.ID,
			chatID:    msg.Chat.ID,
			createdAt: time.Now(),
		})

		c.Respond(&tele.CallbackResponse{Text: "Send me your note."})
		return c.Send("Please type your note and send it as a message:")
	}
}

func handleText(db *sqlx.DB) tele.HandlerFunc {
	return func(c tele.Context) error {
		uid := c.Sender().ID
		text := strings.TrimSpace(c.Text())

		// Check if this is a pending check-in note.
		if val, ok := pendingNotes.LoadAndDelete(uid); ok {
			pn := val.(pendingNote)
			if err := completeCheckin(db, pn.checkinID, text); err != nil {
				log.Printf("completeCheckin (note): %v", err)
				return c.Send("Error completing check-in.")
			}

			now := time.Now().UTC().Format("15:04 UTC")
			// Edit the original ping message.
			editMsg := tele.StoredMessage{MessageID: fmt.Sprintf("%d", pn.msgID), ChatID: pn.chatID}
			c.Bot().Edit(&editMsg, fmt.Sprintf("Checked in at %s with note.", now))
			return sendMainMenu(c, "Checked in with note. Thanks!")
		}

		u, err := getUser(db, uid)
		if err != nil || u.ID == 0 {
			return nil // Ignore text from unregistered users.
		}

		if text == btnCancel.Text {
			pendingActions.Delete(uid)
			return sendMainMenu(c, "Cancelled.")
		}

		if text == btnMenuStatus.Text {
			pendingActions.Delete(uid)
			return handleStatus(db)(c)
		}
		if text == btnMenuSilence.Text {
			pendingActions.Delete(uid)
			return c.Send("Choose how long to silence check-ins.", silenceMenuOptions())
		}
		if text == btnMenuUnsilence.Text {
			pendingActions.Delete(uid)
			return handleUnsilence(db)(c)
		}
		if text == btnMenuMessage.Text {
			pendingActions.Store(uid, pendingAction{
				Kind:      actionComposeMessage,
				CreatedAt: time.Now(),
			})
			return c.Send("Send the message you want to forward to the admins, or tap Cancel.", reasonMenuOptions())
		}
		if text == btnMenuHelp.Text {
			pendingActions.Delete(uid)
			return handleHelp()(c)
		}

		if days := parseSilenceDaysSelection(text); days > 0 {
			pendingActions.Store(uid, pendingAction{
				Kind:      actionSilenceReason,
				Days:      days,
				CreatedAt: time.Now(),
			})
			return c.Send(fmt.Sprintf("Send an optional reason for a %d-day silence, or tap Skip.", days), reasonMenuOptions())
		}

		if text == btnSkip.Text {
			if val, ok := pendingActions.Load(uid); ok {
				action := val.(pendingAction)
				if action.Kind == actionSilenceReason {
					pendingActions.Delete(uid)
					if err := createUserSilence(db, uid, action.Days, ""); err != nil {
						log.Printf("silence skip: %v", err)
						return sendMainMenu(c, "Failed to create silence.")
					}
					return sendMainMenu(c, fmt.Sprintf("Pings silenced for %d day(s).", action.Days))
				}
			}
		}

		if val, ok := pendingActions.Load(uid); ok {
			action := val.(pendingAction)
			switch action.Kind {
			case actionComposeMessage:
				pendingActions.Delete(uid)
				return saveInboxMessage(db, uid, text, c)
			case actionSilenceReason:
				pendingActions.Delete(uid)
				if err := createUserSilence(db, uid, action.Days, text); err != nil {
					log.Printf("silence reason: %v", err)
					return sendMainMenu(c, "Failed to create silence.")
				}
				return sendMainMenu(c, fmt.Sprintf("Pings silenced for %d day(s). Reason: %s", action.Days, text))
			}
		}

		return saveInboxMessage(db, uid, text, c)
	}
}
