package checkinbot

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

const sessionCookieName = "session"
const sessionDuration = 7 * 24 * time.Hour

func requestIsHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	forwarded := r.Header.Get("Forwarded")
	return strings.Contains(strings.ToLower(forwarded), "proto=https")
}

func sessionCookie(r *http.Request, value string, expiresAt time.Time) *http.Cookie {
	secure := requestIsHTTPS(r)
	sameSite := http.SameSiteLaxMode
	if secure {
		// Telegram's hosted login flow may complete through a cross-site popup/redirect.
		// SameSite=None is more reliable there, and requires Secure.
		sameSite = http.SameSiteNoneMode
	}

	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	}
}

// verifyTelegramAuth verifies the data from the Telegram Login Widget.
// It returns the user ID on success.
func verifyTelegramAuth(params map[string]string, botToken string) (int64, error) {
	hash := params["hash"]
	if hash == "" {
		return 0, fmt.Errorf("missing hash")
	}

	// Check auth_date freshness.
	authDateStr := params["auth_date"]
	authDate, err := strconv.ParseInt(authDateStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid auth_date")
	}
	if time.Since(time.Unix(authDate, 0)) > 5*time.Minute {
		return 0, fmt.Errorf("auth data expired")
	}

	// Build data-check string: sort all params except hash, join with \n.
	var keys []string
	for k := range params {
		if k != "hash" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}
	dataCheckString := strings.Join(parts, "\n")

	// secret = SHA256(bot_token)
	secret := sha256.Sum256([]byte(botToken))

	// computed = HMAC-SHA256(data_check_string, secret)
	h := hmac.New(sha256.New, secret[:])
	h.Write([]byte(dataCheckString))
	computed := hex.EncodeToString(h.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(computed), []byte(hash)) != 1 {
		return 0, fmt.Errorf("invalid hash")
	}

	idStr := params["id"]
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid id")
	}

	return userID, nil
}

func generateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func handleTelegramCallback(cfg Config, db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := make(map[string]string)
		for k, v := range r.URL.Query() {
			if len(v) > 0 {
				params[k] = v[0]
			}
		}

		userID, err := verifyTelegramAuth(params, cfg.TelegramToken)
		if err != nil {
			log.Printf("telegram auth failed: %v", err)
			http.Redirect(w, r, "/#/login?error=auth_failed", http.StatusFound)
			return
		}

		// Upsert user from Telegram data.
		upsertUser(db, userID,
			params["username"],
			params["first_name"],
			params["last_name"],
			"",
			false,
			false,
		)

		// Check if user is admin.
		u, err := getUser(db, userID)
		if err == sql.ErrNoRows || !u.IsAdmin {
			http.Redirect(w, r, "/#/login?error=not_admin", http.StatusFound)
			return
		}
		if err != nil {
			log.Printf("getUser: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		// Create session.
		token, err := generateSessionToken()
		if err != nil {
			log.Printf("generateSessionToken: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		expiresAt := time.Now().Add(sessionDuration)
		if err := createSession(db, token, userID, expiresAt); err != nil {
			log.Printf("createSession: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, sessionCookie(r, token, expiresAt))

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func handleLogout(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err == nil {
			deleteSession(db, cookie.Value)
		}

		expiredCookie := sessionCookie(r, "", time.Unix(0, 0))
		expiredCookie.MaxAge = -1
		http.SetCookie(w, expiredCookie)

		http.Redirect(w, r, "/#/login", http.StatusFound)
	}
}

// sessionMiddleware checks for a valid session cookie and injects the user ID.
type contextKey string

const ctxUserID contextKey = "userID"

func sessionMiddleware(db *sqlx.DB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		sess, err := getSession(db, cookie.Value)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		// Verify user is still admin.
		u, err := getUser(db, sess.UserID)
		if err != nil || !u.IsAdmin {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, ctxUserID, sess.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func sessionMiddlewareFunc(db *sqlx.DB, next http.HandlerFunc) http.HandlerFunc {
	return sessionMiddleware(db, next).ServeHTTP
}

// handleDevLogin creates a session for the first admin user in the database,
// bypassing Telegram Login. Only available when --dev is set.
func handleDevLogin(db *sqlx.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admins, err := getAdminUsers(db)
		if err != nil || len(admins) == 0 {
			http.Error(w, "No admin users found. Set --admin-id to seed one.", http.StatusInternalServerError)
			return
		}

		admin := admins[0]
		token, err := generateSessionToken()
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		expiresAt := time.Now().Add(sessionDuration)
		if err := createSession(db, token, admin.ID, expiresAt); err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, sessionCookie(r, token, expiresAt))

		log.Printf("[dev] auto-login as admin %d (%s)", admin.ID, admin.FirstName)
		http.Redirect(w, r, "/", http.StatusFound)
	}
}
