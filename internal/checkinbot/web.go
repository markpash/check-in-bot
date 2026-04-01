package checkinbot

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

func SetupHTTPServer(cfg Config, db *sqlx.DB, botUsername string, spa http.Handler) http.Handler {
	mux := http.NewServeMux()

	// Auth routes (no session required).
	mux.HandleFunc("GET /auth/telegram/callback", handleTelegramCallback(cfg, db))
	mux.HandleFunc("POST /auth/logout", handleLogout(db))
	if cfg.Dev {
		mux.HandleFunc("GET /auth/dev", handleDevLogin(db))
	}

	// API routes (session required).
	api := func(fn http.HandlerFunc) http.HandlerFunc {
		return sessionMiddlewareFunc(db, fn)
	}

	mux.HandleFunc("GET /api/me", api(handleMe(db)))
	mux.HandleFunc("GET /api/dashboard", api(handleDashboard(cfg, db)))
	mux.HandleFunc("GET /api/users", api(handleAPIUsers(db)))
	mux.HandleFunc("GET /api/admins", api(handleAPIAdmins(db)))
	mux.HandleFunc("POST /api/users", api(handleAPICreateUser(db)))
	mux.HandleFunc("GET /api/users/{id}", api(handleAPIUser(db)))
	mux.HandleFunc("POST /api/users/{id}/checkins", api(handleAPISetUserCheckins(db)))
	mux.HandleFunc("POST /api/admins", api(handleAPIAddAdmin(db)))
	mux.HandleFunc("DELETE /api/admins/{id}", api(handleAPIRemoveAdmin(db)))
	mux.HandleFunc("POST /api/users/{id}/note", api(handleAPISetUserNote(db)))
	mux.HandleFunc("POST /api/users/{id}/nickname", api(handleAPISetUserNickname(db)))
	mux.HandleFunc("POST /api/users/{id}/schedule", api(handleAPISetUserSchedule(db)))
	mux.HandleFunc("GET /api/checkins", api(handleAPICheckins(db)))
	mux.HandleFunc("GET /api/silences", api(handleAPISilences(db)))
	mux.HandleFunc("DELETE /api/silences/{id}", api(handleAPIDeleteSilence(db)))
	mux.HandleFunc("GET /api/messages", api(handleAPIMessages(db)))
	mux.HandleFunc("GET /api/messages/{id}", api(handleAPIMessage(db)))
	mux.HandleFunc("POST /api/messages/{id}/read", api(handleAPIMarkRead(db)))
	mux.HandleFunc("POST /api/messages/read-all", api(handleAPIMarkAllRead(db)))
	mux.HandleFunc("GET /api/invites", api(handleAPIInvites(db)))
	mux.HandleFunc("POST /api/invites", api(handleAPICreateInvite(db)))

	// Serve the config needed by the frontend (bot username for Telegram widget).
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"baseURL":     cfg.BaseURL,
			"botUsername": botUsername,
			"dev":         cfg.Dev,
		})
	})

	// Static files — SPA fallback.
	mux.Handle("/", spa)

	return mux
}

// --- API handlers ---

func handleMe(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := r.Context().Value(ctxUserID).(int64)
		u, err := getUser(db, uid)
		if err != nil {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		writeJSON(w, u)
	}
}

func handleDashboard(cfg Config, db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := getDashboard(db, cfg)
		if err != nil {
			log.Printf("dashboard: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		ensureDashboardSlices(&data)
		writeJSON(w, data)
	}
}

func handleAPIUsers(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := getRegularUsers(db)
		if err != nil {
			log.Printf("api users: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		users = ensureSlice(users)
		writeJSON(w, users)
	}
}

func handleAPIAdmins(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admins, err := getAdminUsers(db)
		if err != nil {
			log.Printf("api admins: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		admins = ensureSlice(admins)
		writeJSON(w, admins)
	}
}

func handleAPIUser(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}
		user, err := getUserDetail(db, id)
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("api user: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, user)
	}
}

func handleAPICreateUser(db *sqlx.DB) http.HandlerFunc {
	type request struct {
		ID int64 `json:"id"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == 0 {
			http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
			return
		}
		if err := createManualUser(db, req.ID); err != nil {
			log.Printf("createManualUser: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleAPISetUserCheckins(db *sqlx.DB) http.HandlerFunc {
	type request struct {
		Enabled bool `json:"enabled"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
			return
		}
		if err := setUserCheckinsEnabled(db, id, req.Enabled); err != nil {
			log.Printf("setUserCheckinsEnabled: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleAPIAddAdmin(db *sqlx.DB) http.HandlerFunc {
	type request struct {
		ID int64 `json:"id"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == 0 {
			http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
			return
		}
		if err := addAdminByID(db, req.ID); err != nil {
			log.Printf("addAdminByID: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleAPIRemoveAdmin(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}
		actingUserID := r.Context().Value(ctxUserID).(int64)
		if err := removeAdminByID(db, id, actingUserID); err != nil {
			if strings.Contains(err.Error(), "cannot remove yourself") {
				http.Error(w, `{"error":"cannot remove yourself"}`, http.StatusForbidden)
				return
			}
			log.Printf("removeAdminByID: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleAPISetUserNote(db *sqlx.DB) http.HandlerFunc {
	type request struct {
		Note string `json:"note"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
			return
		}

		if err := setUserNote(db, id, strings.TrimSpace(req.Note)); err != nil {
			log.Printf("setUserNote: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleAPISetUserNickname(db *sqlx.DB) http.HandlerFunc {
	type request struct {
		Nickname string `json:"nickname"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
			return
		}

		if err := setUserNickname(db, id, strings.TrimSpace(req.Nickname)); err != nil {
			log.Printf("setUserNickname: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleAPISetUserSchedule(db *sqlx.DB) http.HandlerFunc {
	type request struct {
		Schedule string `json:"schedule"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}

		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
			return
		}

		schedule := strings.TrimSpace(req.Schedule)
		if schedule != "" {
			if _, err := parseCronSchedule(schedule); err != nil {
				http.Error(w, `{"error":"invalid schedule"}`, http.StatusBadRequest)
				return
			}
		}

		if err := setUserCheckinSchedule(db, id, schedule); err != nil {
			log.Printf("setUserCheckinSchedule: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleAPICheckins(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		date := r.URL.Query().Get("date")
		if date == "" {
			date = time.Now().UTC().Format("2006-01-02")
		}
		checkins, err := getCheckinsByDate(db, date)
		if err != nil {
			log.Printf("api checkins: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		checkins = ensureSlice(checkins)
		writeJSON(w, checkins)
	}
}

func handleAPISilences(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		silences, err := getActiveSilences(db)
		if err != nil {
			log.Printf("api silences: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		silences = ensureSlice(silences)
		writeJSON(w, silences)
	}
}

func handleAPIDeleteSilence(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}
		if err := deleteSilence(db, id); err != nil {
			log.Printf("deleteSilence: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleAPIMessages(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		unread := r.URL.Query().Get("unread") == "true"
		msgs, err := getMessages(db, unread)
		if err != nil {
			log.Printf("api messages: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		msgs = ensureSlice(msgs)
		writeJSON(w, msgs)
	}
}

func handleAPIMessage(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}
		msg, err := getMessageDetail(db, id)
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("api message: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, msg)
	}
}

func handleAPIInvites(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		invites, err := getInviteCodes(db)
		if err != nil {
			log.Printf("api invites: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		invites = ensureSlice(invites)
		writeJSON(w, invites)
	}
}

func handleAPICreateInvite(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actingUserID := r.Context().Value(ctxUserID).(int64)
		code, err := generateInviteCode()
		if err != nil {
			log.Printf("generateInviteCode: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		invite, err := createInviteCode(db, code, actingUserID)
		if err != nil {
			log.Printf("createInviteCode: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, InviteCodeCreateResponse{Invite: invite})
	}
}

func generateInviteCode() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(buf)), nil
}

func handleAPIMarkRead(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
			return
		}
		if err := markMessageRead(db, id); err != nil {
			log.Printf("markMessageRead: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func handleAPIMarkAllRead(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := markAllMessagesRead(db); err != nil {
			log.Printf("markAllMessagesRead: %v", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "ok"})
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func ensureSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func ensureDashboardSlices(data *DashboardResponse) {
	data.Pending = ensureSlice(data.Pending)
	data.Missed = ensureSlice(data.Missed)
	data.CheckedIn = ensureSlice(data.CheckedIn)
	data.Silenced = ensureSlice(data.Silenced)
}
