package checkinbot

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY,
    username      TEXT NOT NULL DEFAULT '',
    first_name    TEXT NOT NULL DEFAULT '',
    last_name     TEXT NOT NULL DEFAULT '',
    nickname      TEXT NOT NULL DEFAULT '',
    checkin_schedule TEXT NOT NULL DEFAULT '',
    language_code TEXT NOT NULL DEFAULT '',
    note          TEXT NOT NULL DEFAULT '',
    is_bot        INTEGER NOT NULL DEFAULT 0,
    is_premium    INTEGER NOT NULL DEFAULT 0,
    is_admin      INTEGER NOT NULL DEFAULT 0,
    checkins_enabled INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS checkins (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id       INTEGER NOT NULL REFERENCES users(id),
    ping_msg_id   INTEGER NOT NULL DEFAULT 0,
    ping_chat_id  INTEGER NOT NULL DEFAULT 0,
    pinged_at     TEXT NOT NULL,
    checked_in_at TEXT,
    invalidated_at TEXT,
    note          TEXT NOT NULL DEFAULT '',
    alerted       INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_checkins_user_id ON checkins(user_id);
CREATE INDEX IF NOT EXISTS idx_checkins_pinged_at ON checkins(pinged_at);

CREATE TABLE IF NOT EXISTS silences (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id       INTEGER NOT NULL REFERENCES users(id),
    days          INTEGER NOT NULL,
    reason        TEXT NOT NULL DEFAULT '',
    starts_at     TEXT NOT NULL DEFAULT (datetime('now')),
    ends_at       TEXT NOT NULL,
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_silences_user_id ON silences(user_id);
CREATE INDEX IF NOT EXISTS idx_silences_ends_at ON silences(ends_at);

CREATE TABLE IF NOT EXISTS messages (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id       INTEGER NOT NULL REFERENCES users(id),
    body          TEXT NOT NULL,
    is_read       INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_messages_user_id ON messages(user_id);

CREATE TABLE IF NOT EXISTS invite_codes (
    code          TEXT PRIMARY KEY,
    created_by    INTEGER NOT NULL REFERENCES users(id),
    used_by       INTEGER REFERENCES users(id),
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    used_at       TEXT
);
CREATE INDEX IF NOT EXISTS idx_invite_codes_used_by ON invite_codes(used_by);

CREATE TABLE IF NOT EXISTS sessions (
    token         TEXT PRIMARY KEY,
    user_id       INTEGER NOT NULL REFERENCES users(id),
    expires_at    TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
`

func OpenDB(path string) *sqlx.DB {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", path)
	db, err := sqlx.Open("sqlite", dsn)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.MustExec(schema)
	ensureUserColumns(db)
	ensureCheckinColumns(db)
	return db
}

func ensureUserColumns(db *sqlx.DB) {
	type userColumn struct {
		Name       string
		Definition string
	}

	columns := []userColumn{
		{Name: "nickname", Definition: "TEXT NOT NULL DEFAULT ''"},
		{Name: "checkin_schedule", Definition: "TEXT NOT NULL DEFAULT ''"},
		{Name: "note", Definition: "TEXT NOT NULL DEFAULT ''"},
		{Name: "language_code", Definition: "TEXT NOT NULL DEFAULT ''"},
		{Name: "is_bot", Definition: "INTEGER NOT NULL DEFAULT 0"},
		{Name: "is_premium", Definition: "INTEGER NOT NULL DEFAULT 0"},
		{Name: "checkins_enabled", Definition: "INTEGER NOT NULL DEFAULT 0"},
	}

	for _, column := range columns {
		var count int
		if err := db.Get(&count, `
			SELECT COUNT(*)
			FROM pragma_table_info('users')
			WHERE name = ?
		`, column.Name); err != nil {
			log.Fatalf("check users.%s column: %v", column.Name, err)
		}
		if count > 0 {
			continue
		}
		query := fmt.Sprintf("ALTER TABLE users ADD COLUMN %s %s", column.Name, column.Definition)
		if _, err := db.Exec(query); err != nil {
			log.Fatalf("add users.%s column: %v", column.Name, err)
		}
	}
}

func ensureCheckinColumns(db *sqlx.DB) {
	type checkinColumn struct {
		Name       string
		Definition string
	}

	columns := []checkinColumn{
		{Name: "invalidated_at", Definition: "TEXT"},
	}

	for _, column := range columns {
		var count int
		if err := db.Get(&count, `
			SELECT COUNT(*)
			FROM pragma_table_info('checkins')
			WHERE name = ?
		`, column.Name); err != nil {
			log.Fatalf("check checkins.%s column: %v", column.Name, err)
		}
		if count > 0 {
			continue
		}
		query := fmt.Sprintf("ALTER TABLE checkins ADD COLUMN %s %s", column.Name, column.Definition)
		if _, err := db.Exec(query); err != nil {
			log.Fatalf("add checkins.%s column: %v", column.Name, err)
		}
	}
}

func SeedAdmins(db *sqlx.DB, id int64) {
	if id == 0 {
		return
	}
	db.MustExec(`
		INSERT INTO users (id, is_admin, checkins_enabled)
		VALUES (?, 1, 1)
		ON CONFLICT(id) DO UPDATE SET is_admin = 1, checkins_enabled = 1, updated_at = datetime('now')
	`, id)
}

// --- User queries ---

func upsertUser(db *sqlx.DB, id int64, username, firstName, lastName, languageCode string, isBot, isPremium bool) {
	db.MustExec(`
		INSERT INTO users (id, username, first_name, last_name, language_code, is_bot, is_premium)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			username = excluded.username,
			first_name = excluded.first_name,
			last_name = excluded.last_name,
			language_code = CASE
				WHEN excluded.language_code <> '' THEN excluded.language_code
				ELSE users.language_code
			END,
			is_bot = excluded.is_bot,
			is_premium = excluded.is_premium,
			updated_at = datetime('now')
	`, id, username, firstName, lastName, languageCode, isBot, isPremium)
}

func getUser(db *sqlx.DB, id int64) (User, error) {
	var u User
	err := db.Get(&u, `SELECT * FROM users WHERE id = ?`, id)
	return u, err
}

func getAllUsers(db *sqlx.DB) ([]User, error) {
	var users []User
	err := db.Select(&users, `SELECT * FROM users ORDER BY created_at DESC`)
	return users, err
}

func getRegularUsers(db *sqlx.DB) ([]User, error) {
	var users []User
	err := db.Select(&users, `SELECT * FROM users WHERE is_admin = 0 ORDER BY created_at DESC`)
	return users, err
}

func setUserCheckinsEnabled(db *sqlx.DB, id int64, enabled bool) error {
	_, err := db.Exec(`UPDATE users SET checkins_enabled = ?, updated_at = datetime('now') WHERE id = ?`, enabled, id)
	return err
}

func setUserAdmin(db *sqlx.DB, id int64, admin bool) error {
	_, err := db.Exec(`UPDATE users SET is_admin = ?, updated_at = datetime('now') WHERE id = ?`, admin, id)
	return err
}

func addAdminByID(db *sqlx.DB, id int64) error {
	_, err := db.Exec(`
		INSERT INTO users (id, first_name, is_admin, checkins_enabled)
		VALUES (?, 'Unknown', 1, 1)
		ON CONFLICT(id) DO UPDATE SET
			is_admin = 1,
			checkins_enabled = 1,
			updated_at = datetime('now')
	`, id)
	return err
}

func createManualUser(db *sqlx.DB, id int64) error {
	_, err := db.Exec(`
		INSERT INTO users (id, first_name, checkins_enabled)
		VALUES (?, 'Unknown', 1)
		ON CONFLICT(id) DO UPDATE SET
			checkins_enabled = 1,
			updated_at = datetime('now')
	`, id)
	return err
}

func removeAdminByID(db *sqlx.DB, id int64, actingUserID int64) error {
	if id == actingUserID {
		return fmt.Errorf("cannot remove yourself as admin")
	}
	_, err := db.Exec(`UPDATE users SET is_admin = 0, updated_at = datetime('now') WHERE id = ?`, id)
	return err
}

func setUserNote(db *sqlx.DB, id int64, note string) error {
	_, err := db.Exec(`UPDATE users SET note = ?, updated_at = datetime('now') WHERE id = ?`, note, id)
	return err
}

func setUserNickname(db *sqlx.DB, id int64, nickname string) error {
	_, err := db.Exec(`UPDATE users SET nickname = ?, updated_at = datetime('now') WHERE id = ?`, nickname, id)
	return err
}

func setUserCheckinSchedule(db *sqlx.DB, id int64, schedule string) error {
	_, err := db.Exec(`UPDATE users SET checkin_schedule = ?, updated_at = datetime('now') WHERE id = ?`, schedule, id)
	return err
}

func getAdminUsers(db *sqlx.DB) ([]User, error) {
	var admins []User
	err := db.Select(&admins, `SELECT * FROM users WHERE is_admin = 1 ORDER BY created_at DESC`)
	return admins, err
}

func getUserDetail(db *sqlx.DB, id int64) (UserDetailResponse, error) {
	var resp UserDetailResponse

	user, err := getUser(db, id)
	if err != nil {
		return resp, err
	}
	resp.User = user

	if err := db.Select(&resp.RecentCheckins, `
		SELECT *
		FROM checkins
		WHERE user_id = ?
		ORDER BY pinged_at DESC
		LIMIT 20
	`, id); err != nil {
		return resp, err
	}

	if err := db.Select(&resp.RecentMessages, `
		SELECT *
		FROM messages
		WHERE user_id = ?
		ORDER BY created_at DESC
		LIMIT 20
	`, id); err != nil {
		return resp, err
	}

	if err := db.Select(&resp.ActiveSilences, `
		SELECT *
		FROM silences
		WHERE user_id = ?
		  AND datetime('now') BETWEEN starts_at AND ends_at
		ORDER BY ends_at ASC
	`, id); err != nil {
		return resp, err
	}

	return resp, nil
}

// --- Check-in queries ---

func getPingableUsers(db *sqlx.DB) ([]User, error) {
	var users []User
	err := db.Select(&users, `
		SELECT u.* FROM users u
		WHERE u.checkins_enabled = 1
		  AND NOT EXISTS (
		    SELECT 1 FROM silences s
		    WHERE s.user_id = u.id
		      AND datetime('now') BETWEEN s.starts_at AND s.ends_at
		  )
	`)
	return users, err
}

func hasCheckinAtTime(db *sqlx.DB, userID int64, t time.Time) (bool, error) {
	var count int
	err := db.Get(&count, `
		SELECT COUNT(*)
		FROM checkins
		WHERE user_id = ?
		  AND strftime('%Y-%m-%d %H:%M', pinged_at) = ?
	`, userID, t.UTC().Format("2006-01-02 15:04"))
	return count > 0, err
}

func createCheckin(db *sqlx.DB, userID int64, msgID int, chatID int64) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO checkins (user_id, ping_msg_id, ping_chat_id, pinged_at)
		VALUES (?, ?, ?, datetime('now'))
	`, userID, msgID, chatID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func completeCheckin(db *sqlx.DB, checkinID int64, note string) error {
	res, err := db.Exec(`
		UPDATE checkins
		SET checked_in_at = datetime('now'), note = ?
		WHERE id = ?
		  AND checked_in_at IS NULL
	`, note, checkinID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func getCheckinByMsg(db *sqlx.DB, msgID int, chatID int64) (CheckIn, error) {
	var c CheckIn
	err := db.Get(&c, `SELECT * FROM checkins WHERE ping_msg_id = ? AND ping_chat_id = ?`, msgID, chatID)
	return c, err
}

func getMissedCheckins(db *sqlx.DB) ([]CheckInWithUser, error) {
	var result []CheckInWithUser
	err := db.Select(&result, `
		SELECT c.*, u.username, u.first_name, u.last_name, u.nickname, u.note, u.checkin_schedule
		FROM checkins c
		JOIN users u ON u.id = c.user_id
		WHERE c.checked_in_at IS NULL
		  AND c.alerted = 0
		  AND u.checkins_enabled = 1
		  AND NOT EXISTS (
		    SELECT 1 FROM silences s
		    WHERE s.user_id = c.user_id
		      AND datetime('now') BETWEEN s.starts_at AND s.ends_at
		  )
		ORDER BY c.pinged_at DESC
	`)
	return result, err
}

func markAlerted(db *sqlx.DB, checkinID int64) error {
	_, err := db.Exec(`UPDATE checkins SET alerted = 1 WHERE id = ?`, checkinID)
	return err
}

func getCheckinsByDate(db *sqlx.DB, date string) ([]CheckInWithUser, error) {
	var result []CheckInWithUser
	err := db.Select(&result, `
		SELECT c.*, u.username, u.first_name, u.last_name, u.nickname, u.note, u.checkin_schedule
		FROM checkins c
		JOIN users u ON u.id = c.user_id
		WHERE date(c.pinged_at) = ?
		ORDER BY c.pinged_at DESC
	`, date)
	return result, err
}

func getDashboard(db *sqlx.DB, cfg Config) (DashboardResponse, error) {
	var resp DashboardResponse

	now := time.Now().UTC().Format("2006-01-02")

	// Checked in today
	var checkedInToday []CheckInWithUser
	if err := db.Select(&checkedInToday, `
		SELECT c.*, u.username, u.first_name, u.last_name, u.nickname, u.note, u.checkin_schedule
		FROM checkins c JOIN users u ON u.id = c.user_id
		WHERE date(c.pinged_at) = ? AND c.checked_in_at IS NOT NULL
		ORDER BY c.checked_in_at DESC
	`, now); err != nil {
		return resp, err
	}
	seenCheckedInUsers := map[int64]bool{}
	for _, checkin := range checkedInToday {
		if seenCheckedInUsers[checkin.UserID] {
			continue
		}
		resp.CheckedIn = append(resp.CheckedIn, checkin)
		seenCheckedInUsers[checkin.UserID] = true
	}

	var open []CheckInWithUser
	if err := db.Select(&open, `
		SELECT c.*, u.username, u.first_name, u.last_name, u.nickname, u.note, u.checkin_schedule
		FROM checkins c JOIN users u ON u.id = c.user_id
		WHERE c.checked_in_at IS NULL
		ORDER BY c.pinged_at DESC
	`); err != nil {
		return resp, err
	}

	nowTime := time.Now().UTC()
	latestMissedByUser := map[int64]CheckInWithUser{}
	for _, checkin := range open {
		missed, err := isCheckinMissed(checkin, cfg, nowTime)
		if err != nil {
			log.Printf("dashboard classify checkin %d: %v", checkin.ID, err)
			continue
		}
		if missed {
			if _, exists := latestMissedByUser[checkin.UserID]; !exists {
				latestMissedByUser[checkin.UserID] = checkin
			}
		} else {
			resp.Pending = append(resp.Pending, checkin)
		}
	}

	for _, checkin := range open {
		if latest, ok := latestMissedByUser[checkin.UserID]; ok && latest.ID == checkin.ID {
			resp.Missed = append(resp.Missed, checkin)
		}
	}

	// Currently silenced
	if err := db.Select(&resp.Silenced, `
		SELECT s.*, u.username, u.first_name, u.last_name, u.nickname, u.note
		FROM silences s JOIN users u ON u.id = s.user_id
		WHERE datetime('now') BETWEEN s.starts_at AND s.ends_at
		ORDER BY s.ends_at ASC
	`); err != nil {
		return resp, err
	}

	return resp, nil
}

// --- Silence queries ---

func createSilence(db *sqlx.DB, userID int64, days int, reason string) error {
	_, err := db.Exec(`
		INSERT INTO silences (user_id, days, reason, ends_at)
		VALUES (?, ?, ?, datetime('now', ? || ' days'))
	`, userID, days, reason, fmt.Sprintf("+%d", days))
	return err
}

func cancelActiveSilences(db *sqlx.DB, userID int64) (int64, error) {
	res, err := db.Exec(`
		DELETE FROM silences WHERE user_id = ? AND datetime('now') BETWEEN starts_at AND ends_at
	`, userID)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func deleteSilence(db *sqlx.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM silences WHERE id = ?`, id)
	return err
}

func getActiveSilences(db *sqlx.DB) ([]SilenceWithUser, error) {
	var result []SilenceWithUser
	err := db.Select(&result, `
		SELECT s.*, u.username, u.first_name, u.last_name, u.nickname, u.note
		FROM silences s JOIN users u ON u.id = s.user_id
		WHERE datetime('now') BETWEEN s.starts_at AND s.ends_at
		ORDER BY s.ends_at ASC
	`)
	return result, err
}

func isUserSilenced(db *sqlx.DB, userID int64) (bool, error) {
	var count int
	err := db.Get(&count, `
		SELECT COUNT(*) FROM silences
		WHERE user_id = ? AND datetime('now') BETWEEN starts_at AND ends_at
	`, userID)
	return count > 0, err
}

// --- Message queries ---

func createMessage(db *sqlx.DB, userID int64, body string) (int64, error) {
	res, err := db.Exec(`INSERT INTO messages (user_id, body) VALUES (?, ?)`, userID, body)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func getMessages(db *sqlx.DB, unreadOnly bool) ([]MessageWithUser, error) {
	var result []MessageWithUser
	q := `
		SELECT m.*, u.username, u.first_name, u.last_name, u.nickname, u.note
		FROM messages m JOIN users u ON u.id = m.user_id
	`
	if unreadOnly {
		q += ` WHERE m.is_read = 0`
	}
	q += ` ORDER BY m.created_at DESC LIMIT 200`
	err := db.Select(&result, q)
	return result, err
}

func getMessageByID(db *sqlx.DB, id int64) (MessageWithUser, error) {
	var msg MessageWithUser
	err := db.Get(&msg, `
		SELECT m.*, u.username, u.first_name, u.last_name, u.nickname, u.note
		FROM messages m
		JOIN users u ON u.id = m.user_id
		WHERE m.id = ?
	`, id)
	return msg, err
}

func getMessageDetail(db *sqlx.DB, id int64) (MessageDetailResponse, error) {
	var resp MessageDetailResponse

	msg, err := getMessageByID(db, id)
	if err != nil {
		return resp, err
	}
	user, err := getUser(db, msg.UserID)
	if err != nil {
		return resp, err
	}

	resp.Message = msg
	resp.User = user
	return resp, nil
}

func createInviteCode(db *sqlx.DB, code string, createdBy int64) (InviteCodeWithUser, error) {
	if _, err := db.Exec(`
		INSERT INTO invite_codes (code, created_by)
		VALUES (?, ?)
	`, code, createdBy); err != nil {
		return InviteCodeWithUser{}, err
	}
	return getInviteCode(db, code)
}

func getInviteCode(db *sqlx.DB, code string) (InviteCodeWithUser, error) {
	var invite InviteCodeWithUser
	err := db.Get(&invite, `
		SELECT i.*,
			COALESCE(u.username, '') AS used_by_username,
			COALESCE(u.first_name, '') AS used_by_first_name,
			COALESCE(u.last_name, '') AS used_by_last_name,
			COALESCE(u.nickname, '') AS used_by_nickname
		FROM invite_codes i
		LEFT JOIN users u ON u.id = i.used_by
		WHERE i.code = ?
	`, code)
	return invite, err
}

func getInviteCodes(db *sqlx.DB) ([]InviteCodeWithUser, error) {
	var invites []InviteCodeWithUser
	err := db.Select(&invites, `
		SELECT i.*,
			COALESCE(u.username, '') AS used_by_username,
			COALESCE(u.first_name, '') AS used_by_first_name,
			COALESCE(u.last_name, '') AS used_by_last_name,
			COALESCE(u.nickname, '') AS used_by_nickname
		FROM invite_codes i
		LEFT JOIN users u ON u.id = i.used_by
		ORDER BY i.created_at DESC
	`)
	return invites, err
}

func redeemInviteCode(db *sqlx.DB, code string, userID int64) (bool, error) {
	tx, err := db.Beginx()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var invite InviteCode
	if err := tx.Get(&invite, `SELECT * FROM invite_codes WHERE code = ?`, code); err != nil {
		return false, err
	}
	if invite.UsedBy != nil {
		return false, nil
	}

	if _, err := tx.Exec(`
		UPDATE invite_codes
		SET used_by = ?, used_at = datetime('now')
		WHERE code = ? AND used_by IS NULL
	`, userID, code); err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func markMessageRead(db *sqlx.DB, id int64) error {
	_, err := db.Exec(`UPDATE messages SET is_read = 1 WHERE id = ?`, id)
	return err
}

func markAllMessagesRead(db *sqlx.DB) error {
	_, err := db.Exec(`UPDATE messages SET is_read = 1 WHERE is_read = 0`)
	return err
}

// --- Session queries ---

func createSession(db *sqlx.DB, token string, userID int64, expiresAt time.Time) error {
	_, err := db.Exec(`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, expiresAt.UTC().Format("2006-01-02 15:04:05"))
	return err
}

func getSession(db *sqlx.DB, token string) (Session, error) {
	var s Session
	err := db.Get(&s, `SELECT * FROM sessions WHERE token = ? AND expires_at > datetime('now')`, token)
	return s, err
}

func deleteSession(db *sqlx.DB, token string) error {
	_, err := db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

func cleanExpiredSessions(db *sqlx.DB) {
	db.Exec(`DELETE FROM sessions WHERE expires_at < datetime('now')`)
}
