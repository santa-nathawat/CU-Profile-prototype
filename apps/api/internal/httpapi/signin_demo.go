package httpapi

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/thinc-org/CU-Profile/apps/api/internal/demo"
)

// The demo sign-in provider. Chula OAuth replaces this file: it verifies the
// identity, then calls s.signIn with the same email and its own subject.

var chulaStudentEmail = regexp.MustCompile(`^[^@\s]+@student\.chula\.ac\.th$`)

func (s *Server) registerDemoSignIn() {
	if !s.demoSignIn {
		return
	}
	s.handle("POST /auth/demo-sign-in", s.demoSignInHandler)
	s.handle("GET /auth/demo-accounts", s.demoAccounts)
}

func (s *Server) demoSignInHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if !chulaStudentEmail.MatchString(email) {
		writeError(w, http.StatusBadRequest, "not_chula_email", "Only @student.chula.ac.th emails can sign in.")
		return
	}
	user, err := s.signIn(r.Context(), w, email, "demo:"+email)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	me, err := s.loadMe(r.Context(), user)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, me)
}

// demoAccounts lists the demo accounts that have been seeded and are still open.
func (s *Server) demoAccounts(w http.ResponseWriter, r *http.Request) {
	type item struct {
		Email       string `json:"email"`
		DisplayName string `json:"displayName"`
		Note        string `json:"note"`
	}
	rows, err := s.db.Query(r.Context(), `
		SELECT u.email, COALESCE(p.display_name, 'New student')
		FROM users u LEFT JOIN profiles p ON p.user_id = u.id
		WHERE u.email = ANY($1) AND u.closed_at IS NULL`, demo.Emails())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	defer rows.Close()
	names := map[string]string{}
	for rows.Next() {
		var email, name string
		if err := rows.Scan(&email, &name); err != nil {
			writeErr(w, r, err)
			return
		}
		names[email] = name
	}
	if err := rows.Err(); err != nil {
		writeErr(w, r, err)
		return
	}
	items := []item{}
	for _, account := range demo.Accounts {
		if name, seeded := names[account.Email]; seeded {
			items = append(items, item{account.Email, name, account.Note})
		}
	}
	writeJSON(w, http.StatusOK, map[string][]item{"items": items})
}
