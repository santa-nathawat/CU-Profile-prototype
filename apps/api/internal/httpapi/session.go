package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	sessionCookie   = "cu_session"
	sessionLifetime = 30 * 24 * time.Hour
)

// User is the signed-in student behind a request.
type User struct {
	ID    string
	Email string
}

type userHandler func(w http.ResponseWriter, r *http.Request, user User)

// requireUser answers 401 unless the request has a live session on an open account.
func (s *Server) requireUser(next userHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			writeNotSignedIn(w)
			return
		}
		var user User
		err = s.db.QueryRow(r.Context(), `
			SELECT u.id, u.email
			FROM sessions s JOIN users u ON u.id = s.user_id
			WHERE s.token_hash = $1 AND s.revoked_at IS NULL AND s.expires_at > now() AND u.closed_at IS NULL`,
			hashToken(cookie.Value)).Scan(&user.ID, &user.Email)
		if errors.Is(err, pgx.ErrNoRows) {
			writeNotSignedIn(w)
			return
		}
		if err != nil {
			writeErr(w, r, err)
			return
		}
		next(w, r, user)
	}
}

func writeNotSignedIn(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "not_signed_in", "Please sign in to continue.")
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// signIn finds the account by its provider subject, creating it on first sign-in,
// and starts a session. Sign-in providers call this once they trust the identity.
func (s *Server) signIn(ctx context.Context, w http.ResponseWriter, email, subject string) (User, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return User{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	user := User{Email: email}
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		// The WHERE makes a closed account return no row, so it can't sign back in.
		err := tx.QueryRow(ctx, `
			INSERT INTO users (email, oauth_subject, last_signed_in_at) VALUES ($1, $2, now())
			ON CONFLICT (oauth_subject) DO UPDATE SET last_signed_in_at = now(), updated_at = now()
			WHERE users.closed_at IS NULL
			RETURNING id`, email, subject).Scan(&user.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return forbidden("account_closed", "This account has been closed.")
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO sessions (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
			user.ID, hashToken(token), time.Now().Add(sessionLifetime))
		return err
	})
	if err != nil {
		return User{}, err
	}
	s.setSessionCookie(w, token, int(sessionLifetime.Seconds()))
	return user, nil
}

func (s *Server) setSessionCookie(w http.ResponseWriter, value string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: value, Path: "/", MaxAge: maxAge,
		HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.cookieSecure,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	s.setSessionCookie(w, "", -1)
}

func (s *Server) signOut(w http.ResponseWriter, r *http.Request) {
	if err := readEmptyJSON(w, r); err != nil {
		writeErr(w, r, err)
		return
	}
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		_, err := s.db.Exec(r.Context(), `UPDATE sessions SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, hashToken(cookie.Value))
		if err != nil {
			writeErr(w, r, err)
			return
		}
	}
	s.clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}
