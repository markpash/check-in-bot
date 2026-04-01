package checkinbot

import (
	"strconv"
)

type User struct {
	ID              int64  `db:"id"            json:"id"`
	Username        string `db:"username"      json:"username"`
	FirstName       string `db:"first_name"    json:"firstName"`
	LastName        string `db:"last_name"     json:"lastName"`
	Nickname        string `db:"nickname"      json:"nickname"`
	LanguageCode    string `db:"language_code" json:"languageCode"`
	Note            string `db:"note"          json:"note"`
	IsBot           bool   `db:"is_bot"        json:"isBot"`
	IsPremium       bool   `db:"is_premium"    json:"isPremium"`
	IsAdmin         bool   `db:"is_admin"      json:"isAdmin"`
	CheckinsEnabled bool   `db:"checkins_enabled" json:"checkinsEnabled"`
	CheckinSchedule string `db:"checkin_schedule" json:"checkinSchedule"`
	CreatedAt       string `db:"created_at"    json:"createdAt"`
	UpdatedAt       string `db:"updated_at"    json:"updatedAt"`
}

// Recipient implements telebot.Recipient.
func (u User) Recipient() string {
	return strconv.FormatInt(u.ID, 10)
}

type CheckIn struct {
	ID          int64   `db:"id"            json:"id"`
	UserID      int64   `db:"user_id"       json:"userId"`
	PingMsgID   int     `db:"ping_msg_id"   json:"-"`
	PingChatID  int64   `db:"ping_chat_id"  json:"-"`
	PingedAt    string  `db:"pinged_at"     json:"pingedAt"`
	CheckedInAt *string `db:"checked_in_at" json:"checkedInAt"`
	InvalidatedAt *string `db:"invalidated_at" json:"-"`
	Note        string  `db:"note"          json:"note"`
	Alerted     bool    `db:"alerted"       json:"-"`
	CreatedAt   string  `db:"created_at"    json:"createdAt"`
}

type Silence struct {
	ID        int64  `db:"id"         json:"id"`
	UserID    int64  `db:"user_id"    json:"userId"`
	Days      int    `db:"days"       json:"days"`
	Reason    string `db:"reason"     json:"reason"`
	StartsAt  string `db:"starts_at"  json:"startsAt"`
	EndsAt    string `db:"ends_at"    json:"endsAt"`
	CreatedAt string `db:"created_at" json:"createdAt"`
}

type Message struct {
	ID        int64  `db:"id"         json:"id"`
	UserID    int64  `db:"user_id"    json:"userId"`
	Body      string `db:"body"       json:"body"`
	IsRead    bool   `db:"is_read"    json:"isRead"`
	CreatedAt string `db:"created_at" json:"createdAt"`
}

type InviteCode struct {
	Code      string  `db:"code"       json:"code"`
	CreatedBy int64   `db:"created_by" json:"createdBy"`
	UsedBy    *int64  `db:"used_by"    json:"usedBy"`
	CreatedAt string  `db:"created_at" json:"createdAt"`
	UsedAt    *string `db:"used_at"    json:"usedAt"`
}

type Session struct {
	Token     string `db:"token"`
	UserID    int64  `db:"user_id"`
	ExpiresAt string `db:"expires_at"`
}

// TelegramRecipient implements telebot.Recipient for sending to arbitrary user IDs.
type TelegramRecipient int64

func (r TelegramRecipient) Recipient() string {
	return strconv.FormatInt(int64(r), 10)
}

// Joined types for queries that join across tables.

type CheckInWithUser struct {
	CheckIn
	Username        string `db:"username"         json:"username"`
	FirstName       string `db:"first_name"       json:"firstName"`
	LastName        string `db:"last_name"        json:"lastName"`
	Nickname        string `db:"nickname"         json:"nickname"`
	Note            string `db:"note"             json:"note"`
	CheckinSchedule string `db:"checkin_schedule" json:"checkinSchedule"`
}

type SilenceWithUser struct {
	Silence
	Username  string `db:"username"   json:"username"`
	FirstName string `db:"first_name" json:"firstName"`
	LastName  string `db:"last_name"  json:"lastName"`
	Nickname  string `db:"nickname"   json:"nickname"`
	Note      string `db:"note"       json:"note"`
}

type MessageWithUser struct {
	Message
	Username  string `db:"username"   json:"username"`
	FirstName string `db:"first_name" json:"firstName"`
	LastName  string `db:"last_name"  json:"lastName"`
	Nickname  string `db:"nickname"   json:"nickname"`
	Note      string `db:"note"       json:"note"`
}

type InviteCodeWithUser struct {
	InviteCode
	UsedByUsername  string `db:"used_by_username"   json:"usedByUsername"`
	UsedByFirstName string `db:"used_by_first_name" json:"usedByFirstName"`
	UsedByLastName  string `db:"used_by_last_name"  json:"usedByLastName"`
	UsedByNickname  string `db:"used_by_nickname"   json:"usedByNickname"`
}

type DashboardResponse struct {
	Pending   []CheckInWithUser `json:"pending"`
	Missed    []CheckInWithUser `json:"missed"`
	CheckedIn []CheckInWithUser `json:"checkedIn"`
	Silenced  []SilenceWithUser `json:"silenced"`
}

type UserDetailResponse struct {
	User           User      `json:"user"`
	RecentCheckins []CheckIn `json:"recentCheckins"`
	RecentMessages []Message `json:"recentMessages"`
	ActiveSilences []Silence `json:"activeSilences"`
}

type MessageDetailResponse struct {
	Message MessageWithUser `json:"message"`
	User    User            `json:"user"`
}

type InviteCodeCreateResponse struct {
	Invite InviteCodeWithUser `json:"invite"`
}
