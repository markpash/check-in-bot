package checkinbot

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	tele "gopkg.in/telebot.v4"
)

func RunPinger(ctx context.Context, bot *tele.Bot, db *sqlx.DB, cfg Config) {
	for {
		next := time.Now().UTC().Truncate(time.Minute).Add(time.Minute)
		log.Printf("next check-in evaluation at %s", next.Format(time.RFC3339))

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
			sendDuePings(bot, db, cfg, next)
		}
	}
}

func sendDuePings(bot *tele.Bot, db *sqlx.DB, cfg Config, now time.Time) {
	users, err := getPingableUsers(db)
	if err != nil {
		log.Printf("sendDuePings: getPingableUsers: %v", err)
		return
	}

	sentCount := 0
	for _, u := range users {
		schedule := strings.TrimSpace(u.CheckinSchedule)
		if schedule == "" {
			schedule = cfg.CheckInSchedule
		}

		parsed, err := parseCronSchedule(schedule)
		if err != nil {
			log.Printf("sendDuePings: invalid schedule for user %d: %v", u.ID, err)
			continue
		}
		if !parsed.matches(now.UTC()) {
			continue
		}

		already, err := hasCheckinAtTime(db, u.ID, now)
		if err != nil {
			log.Printf("sendDuePings: hasCheckinAtTime(%d): %v", u.ID, err)
			continue
		}
		if already {
			continue
		}

		msg, err := bot.Send(u, "Time for your check-in.", checkInMarkup)
		if err != nil {
			log.Printf("sendDuePings: send to %d: %v", u.ID, err)
			continue
		}

		_, err = createCheckin(db, u.ID, msg.ID, msg.Chat.ID)
		if err != nil {
			log.Printf("sendDuePings: createCheckin(%d): %v", u.ID, err)
			continue
		}
		sentCount++
	}

	log.Printf("check-in pings sent to %d users", sentCount)
}

func effectiveCheckinSchedule(schedule string, cfg Config) string {
	if strings.TrimSpace(schedule) != "" {
		return strings.TrimSpace(schedule)
	}
	return cfg.CheckInSchedule
}

func isCheckinMissed(checkin CheckInWithUser, cfg Config, now time.Time) (bool, error) {
	pingedAt, err := parseDBTime(checkin.PingedAt)
	if err != nil {
		return false, err
	}
	nextDue, err := nextCronTime(effectiveCheckinSchedule(checkin.CheckinSchedule, cfg), pingedAt)
	if err != nil {
		return false, err
	}
	return !now.UTC().Before(nextDue), nil
}

func RunMissedMonitor(ctx context.Context, bot *tele.Bot, db *sqlx.DB, cfg Config) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkForMissedCheckins(bot, db, cfg)
			cleanExpiredSessions(db)
		}
	}
}

func checkForMissedCheckins(bot *tele.Bot, db *sqlx.DB, cfg Config) {
	missed, err := getMissedCheckins(db)
	if err != nil {
		log.Printf("checkForMissedCheckins: %v", err)
		return
	}

	if len(missed) == 0 {
		return
	}

	admins, err := getAdminUsers(db)
	if err != nil {
		log.Printf("checkForMissedCheckins: getAdminUsers: %v", err)
		return
	}

	now := time.Now().UTC()
	sent := 0
	for _, m := range missed {
		isMissed, err := isCheckinMissed(m, cfg, now)
		if err != nil {
			log.Printf("checkForMissedCheckins classify %d: %v", m.ID, err)
			continue
		}
		if !isMissed {
			continue
		}

		name := m.FirstName
		if m.Username != "" {
			name += fmt.Sprintf(" (@%s)", m.Username)
		}

		pingedAt, _ := time.Parse("2006-01-02 15:04:05", m.PingedAt)
		hoursAgo := time.Since(pingedAt).Hours()

		alert := fmt.Sprintf("Alert: %s has not checked in. Pinged %.0f hours ago.", name, hoursAgo)

		for _, admin := range admins {
			if _, err := bot.Send(admin, alert); err != nil {
				log.Printf("alert admin %d: %v", admin.ID, err)
			}
		}

		markAlerted(db, m.ID)
		sent++
	}

	log.Printf("sent %d missed check-in alerts", sent)
}
