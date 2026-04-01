package checkinbot

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v4"
)

type Config struct {
	TelegramToken string
	DBPath        string
	ListenAddr    string
	BaseURL       string
	CheckInSchedule string
	AdminID       int64
	Dev           bool
}

func ParseConfig() (Config, error) {
	fs := ff.NewFlagSet("check-in-bot")

	token := fs.String('t', "token", "", "Telegram bot token")
	dbPath := fs.String('d', "db", "checkin.db", "SQLite database path")
	listen := fs.String('l', "listen", ":8080", "HTTP listen address")
	baseURL := fs.String('b', "base-url", "http://localhost:8080", "Public base URL for the web UI")
	checkinSchedule := fs.String(0, "checkin-schedule", "0 9 * * *", "UTC cron schedule for check-in pings (5 fields: minute hour day-of-month month day-of-week)")
	adminID := fs.String(0, "admin-id", "", "Telegram user ID for the seed admin")
	dev := fs.Bool(0, "dev", "Enable dev mode (bypasses Telegram Login for web UI)")

	err := ff.Parse(fs, os.Args[1:], ff.WithEnvVarPrefix("CHECKIN"))
	if err != nil {
		return Config{}, err
	}

	if *token == "" {
		return Config{}, fmt.Errorf("--token (or CHECKIN_TOKEN) is required")
	}

	cfg := Config{
		TelegramToken: *token,
		DBPath:        *dbPath,
		ListenAddr:    *listen,
		BaseURL:       strings.TrimRight(*baseURL, "/"),
		CheckInSchedule: strings.TrimSpace(*checkinSchedule),
		Dev:           *dev,
	}

	if _, err := parseCronSchedule(cfg.CheckInSchedule); err != nil {
		return Config{}, fmt.Errorf("invalid --checkin-schedule: %w", err)
	}

	if *adminID != "" {
		id, err := strconv.ParseInt(strings.TrimSpace(*adminID), 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("invalid admin ID %q: %w", *adminID, err)
		}
		cfg.AdminID = id
	}

	return cfg, nil
}
