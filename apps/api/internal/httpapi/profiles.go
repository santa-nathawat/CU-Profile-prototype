package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const (
	maxDisplayName  = 80
	maxHeadline     = 160
	maxWorkingStyle = 1000
	maxSkillName    = 50
	maxSkills       = 30
	cardSkillLimit  = 5
)

func (s *Server) registerProfiles() {
	s.handle("GET /me/profile", s.requireUser(s.getMyProfile))
	s.handle("PUT /me/profile", s.requireUser(s.putMyProfile))
	s.handle("PUT /me/availability", s.requireUser(s.putAvailability))
	s.handle("GET /profiles", s.requireUser(s.listProfiles))
	s.handle("GET /profiles/{id}", s.requireUser(s.getProfile))
}

type Rating struct {
	Average *float64 `json:"average"`
	Count   int      `json:"count"`
}

type Slot struct {
	Day   int    `json:"day"`
	Start string `json:"start"`
}

type profileCV struct {
	Available bool    `json:"available"`
	FileName  *string `json:"fileName"`
}

type invitableRole struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Available int    `json:"available"`
}

type invitableProject struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Roles []invitableRole `json:"roles"`
}

type viewerActions struct {
	InvitableProjects []invitableProject `json:"invitableProjects"`
}

type Profile struct {
	UserID        string        `json:"userId"`
	DisplayName   *string       `json:"displayName"`
	Faculty       *idName       `json:"faculty"`
	Year          *int          `json:"year"`
	Headline      *string       `json:"headline"`
	WorkingStyle  *string       `json:"workingStyle"`
	Skills        []string      `json:"skills"`
	Availability  []Slot        `json:"availability"`
	Rating        Rating        `json:"rating"`
	IsSelf        bool          `json:"isSelf"`
	CV            profileCV     `json:"cv"`
	ViewerActions viewerActions `json:"viewerActions"`
}

type MyProfile struct {
	Profile
	Email      string `json:"email"`
	CVShared   bool   `json:"cvShared"`
	Searchable bool   `json:"searchable"`
}

// loadedProfile is a Profile plus the columns only the server decides on.
type loadedProfile struct {
	Profile
	CVShared   bool
	Searchable bool
	Closed     bool
}

// loadProfile reads the profile as viewerID may see it. A user with no profile row
// yet gets an empty profile, which is how the first-run form starts.
func loadProfile(ctx context.Context, q querier, userID, viewerID string) (loadedProfile, error) {
	var out loadedProfile
	var facultyID, facultyName, cvFileName *string
	err := q.QueryRow(ctx, `
		SELECT u.id, u.closed_at IS NOT NULL, p.display_name, f.id, f.name, p.year, p.headline, p.working_style,
			p.cv_file_name, COALESCE(p.cv_shared, false), `+searchableSQL+`, rating.average, rating.count
		FROM users u
		LEFT JOIN profiles p ON p.user_id = u.id
		LEFT JOIN faculties f ON f.id = p.faculty_id
		`+ratingJoinSQL("u.id")+`
		WHERE u.id = $1`, userID).Scan(
		&out.UserID, &out.Closed, &out.DisplayName, &facultyID, &facultyName, &out.Year, &out.Headline, &out.WorkingStyle,
		&cvFileName, &out.CVShared, &out.Searchable, &out.Rating.Average, &out.Rating.Count)
	if err != nil {
		return out, err
	}
	if facultyID != nil {
		out.Faculty = &idName{*facultyID, *facultyName}
	}
	out.IsSelf = userID == viewerID
	if cvFileName != nil && (out.IsSelf || out.CVShared) {
		out.CV = profileCV{Available: true, FileName: cvFileName}
	}
	if out.Skills, err = loadSkills(ctx, q, userID); err != nil {
		return out, err
	}
	if out.Availability, err = loadAvailability(ctx, q, userID); err != nil {
		return out, err
	}
	out.ViewerActions.InvitableProjects = []invitableProject{}
	if !out.IsSelf {
		out.ViewerActions.InvitableProjects, err = loadInvitableProjects(ctx, q, viewerID, userID)
	}
	return out, err
}

// Skills keep the order the student listed them in, so the first few are their top skills.
func loadSkills(ctx context.Context, q querier, userID string) ([]string, error) {
	rows, err := q.Query(ctx, `
		SELECT s.name FROM user_skills us JOIN skills s ON s.id = us.skill_id
		WHERE us.user_id = $1 ORDER BY us.created_at, s.name`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func loadAvailability(ctx context.Context, q querier, userID string) ([]Slot, error) {
	rows, err := q.Query(ctx, `
		SELECT day_of_week, to_char(start_time, 'HH24:MI') FROM availability_slots
		WHERE user_id = $1 ORDER BY day_of_week, start_time`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Slot, error) {
		var slot Slot
		err := row.Scan(&slot.Day, &slot.Start)
		return slot, err
	})
}

// loadInvitableProjects leaves out teams the student is already on, since inviting them can only fail.
func loadInvitableProjects(ctx context.Context, q querier, viewerID, studentID string) ([]invitableProject, error) {
	rows, err := q.Query(ctx, `
		SELECT pr.id, pr.name, r.id, r.name,
			GREATEST(r.capacity - (SELECT count(*) FROM project_members m WHERE m.role_id = r.id AND m.left_at IS NULL), 0)
		FROM projects pr JOIN project_roles r ON r.project_id = pr.id AND r.removed_at IS NULL
		WHERE pr.lead_user_id = $1 AND pr.recruitment_status = 'open' AND pr.deleted_at IS NULL
			AND pr.status IN ('not_started', 'in_progress')
			AND NOT EXISTS (SELECT 1 FROM project_members m WHERE m.project_id = pr.id AND m.user_id = $2 AND m.left_at IS NULL)
		ORDER BY pr.created_at DESC, pr.id, r.created_at, r.id`, viewerID, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := []invitableProject{}
	for rows.Next() {
		var projectID, projectName string
		var role invitableRole
		if err := rows.Scan(&projectID, &projectName, &role.ID, &role.Name, &role.Available); err != nil {
			return nil, err
		}
		if len(projects) == 0 || projects[len(projects)-1].ID != projectID {
			projects = append(projects, invitableProject{ID: projectID, Name: projectName})
		}
		last := &projects[len(projects)-1]
		last.Roles = append(last.Roles, role)
	}
	return projects, rows.Err()
}

func (s *Server) loadMyProfile(ctx context.Context, q querier, user User) (MyProfile, error) {
	loaded, err := loadProfile(ctx, q, user.ID, user.ID)
	if err != nil {
		return MyProfile{}, err
	}
	return MyProfile{Profile: loaded.Profile, Email: user.Email, CVShared: loaded.CVShared, Searchable: loaded.Searchable}, nil
}

func (s *Server) getMyProfile(w http.ResponseWriter, r *http.Request, user User) {
	profile, err := s.loadMyProfile(r.Context(), s.db, user)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

var errProfileNotFound = notFound("profile_not_found", "We couldn't find that profile.")

// visibleProfileID checks the {id} path value against the 404 rule for other
// people's profiles: closed accounts and those without a display name don't exist.
func (s *Server) visibleProfileID(r *http.Request) (string, error) {
	id, ok := pathID(r, "id")
	if !ok {
		return "", errProfileNotFound
	}
	var visible bool
	err := s.db.QueryRow(r.Context(), `
		SELECT EXISTS (SELECT 1 FROM users u JOIN profiles p ON p.user_id = u.id
			WHERE u.id = $1 AND u.closed_at IS NULL AND p.display_name IS NOT NULL)`, id).Scan(&visible)
	if err != nil {
		return "", err
	}
	if !visible {
		return "", errProfileNotFound
	}
	return id, nil
}

func (s *Server) getProfile(w http.ResponseWriter, r *http.Request, user User) {
	id, err := s.visibleProfileID(r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	loaded, err := loadProfile(r.Context(), s.db, id, user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, loaded.Profile)
}

type profileInput struct {
	DisplayName  *string  `json:"displayName"`
	FacultyID    *string  `json:"facultyId"`
	Year         *int     `json:"year"`
	Headline     *string  `json:"headline"`
	WorkingStyle *string  `json:"workingStyle"`
	Skills       []string `json:"skills"`
}

// cleanText trims a nullable text field. Empty becomes null, so "Not specified" has one form.
func cleanText(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func checkLength(fields map[string]string, key string, value *string, limit int) {
	if value != nil && utf8.RuneCountInString(*value) > limit {
		fields[key] = fmt.Sprintf("Keep this to %d characters or fewer.", limit)
	}
}

// normalize tidies the input and reports what's wrong with it, by field.
func (in *profileInput) normalize() map[string]string {
	fields := map[string]string{}
	in.DisplayName = cleanText(in.DisplayName)
	in.Headline = cleanText(in.Headline)
	in.WorkingStyle = cleanText(in.WorkingStyle)
	in.FacultyID = cleanText(in.FacultyID)
	checkLength(fields, "displayName", in.DisplayName, maxDisplayName)
	checkLength(fields, "headline", in.Headline, maxHeadline)
	checkLength(fields, "workingStyle", in.WorkingStyle, maxWorkingStyle)
	if in.Year != nil && (*in.Year < 1 || *in.Year > 8) {
		fields["year"] = "Year must be from 1 to 8."
	}
	if in.FacultyID != nil && !validUUID(*in.FacultyID) {
		fields["facultyId"] = "Choose a faculty from the list."
	}
	seen := map[string]bool{}
	skills := []string{}
	for _, raw := range in.Skills {
		name := strings.Join(strings.Fields(raw), " ")
		key := strings.ToLower(name)
		if name == "" || seen[key] {
			continue
		}
		seen[key] = true
		if utf8.RuneCountInString(name) > maxSkillName {
			fields["skills"] = fmt.Sprintf("Keep each skill to %d characters or fewer.", maxSkillName)
		}
		skills = append(skills, name)
	}
	if len(skills) > maxSkills {
		fields["skills"] = fmt.Sprintf("Add %d skills or fewer.", maxSkills)
	}
	in.Skills = skills
	return fields
}

func (s *Server) putMyProfile(w http.ResponseWriter, r *http.Request, user User) {
	var in profileInput
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, r, err)
		return
	}
	if fields := in.normalize(); len(fields) > 0 {
		writeErr(w, r, invalidFields(fields))
		return
	}
	ctx := r.Context()
	var profile MyProfile
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if in.FacultyID != nil {
			var exists bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM faculties WHERE id = $1)`, *in.FacultyID).Scan(&exists); err != nil {
				return err
			}
			if !exists {
				return invalidFields(map[string]string{"facultyId": "Choose a faculty from the list."})
			}
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO profiles (user_id, display_name, faculty_id, year, headline, working_style)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (user_id) DO UPDATE SET
				display_name = EXCLUDED.display_name, faculty_id = EXCLUDED.faculty_id, year = EXCLUDED.year,
				headline = EXCLUDED.headline, working_style = EXCLUDED.working_style, updated_at = now()`,
			user.ID, in.DisplayName, in.FacultyID, in.Year, in.Headline, in.WorkingStyle)
		if err != nil {
			return err
		}
		if err := replaceSkills(ctx, tx, user.ID, in.Skills); err != nil {
			return err
		}
		profile, err = s.loadMyProfile(ctx, tx, user)
		return err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

// replaceSkills matches names on normalized_name and creates unknown ones as custom skills.
func replaceSkills(ctx context.Context, tx pgx.Tx, userID string, names []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM user_skills WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if len(names) == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO skills (name) SELECT unnest($1::text[])
		ON CONFLICT (normalized_name) DO NOTHING`, names); err != nil {
		return err
	}
	// created_at carries the listed order, because every row in a transaction would otherwise share one now().
	_, err := tx.Exec(ctx, `
		INSERT INTO user_skills (user_id, skill_id, created_at)
		SELECT $1, s.id, now() + listed.position * interval '1 millisecond'
		FROM unnest($2::text[]) WITH ORDINALITY AS listed(name, position)
		JOIN skills s ON s.normalized_name = lower(trim(listed.name))`, userID, names)
	return err
}

var slotStart = regexp.MustCompile(`^([01][0-9]|2[0-3]):(00|30)$`)

func (s *Server) putAvailability(w http.ResponseWriter, r *http.Request, user User) {
	var body struct {
		Slots []Slot `json:"slots"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	seen := map[Slot]bool{}
	days := []int16{}
	starts := []string{}
	for _, slot := range body.Slots {
		if slot.Day < 1 || slot.Day > 7 || !slotStart.MatchString(slot.Start) {
			writeErr(w, r, badRequest("invalid_slot", "Each slot needs a day from 1 to 7 and a start time on :00 or :30."))
			return
		}
		if !seen[slot] {
			seen[slot] = true
			days = append(days, int16(slot.Day))
			starts = append(starts, slot.Start)
		}
	}
	ctx := r.Context()
	var slots []Slot
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM availability_slots WHERE user_id = $1`, user.ID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO availability_slots (user_id, day_of_week, start_time)
			SELECT $1, slot.day, slot.start::time FROM unnest($2::smallint[], $3::text[]) AS slot(day, start)`,
			user.ID, days, starts)
		if err != nil {
			return err
		}
		slots, err = loadAvailability(ctx, tx, user.ID)
		return err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]Slot{"slots": slots})
}

type ProfileCard struct {
	UserID      string   `json:"userId"`
	DisplayName string   `json:"displayName"`
	Faculty     string   `json:"faculty"`
	Year        *int     `json:"year"`
	Headline    *string  `json:"headline"`
	Skills      []string `json:"skills"`
	Rating      Rating   `json:"rating"`
}

func (s *Server) listProfiles(w http.ResponseWriter, r *http.Request, _ User) {
	ctx := r.Context()
	req := parsePage(r)
	term := strings.TrimSpace(r.URL.Query().Get("q"))
	matchName := term != ""
	if !matchName {
		term = strings.TrimSpace(r.URL.Query().Get("skill"))
	}
	var pattern *string
	if term != "" {
		like := likePattern(term)
		pattern = &like
	}
	const from = `
		FROM profiles p
		JOIN users u ON u.id = p.user_id
		JOIN faculties f ON f.id = p.faculty_id
		WHERE ` + searchableSQL + ` AND ($1::text IS NULL OR ($2::boolean AND p.display_name ILIKE $1) OR EXISTS (
			SELECT 1 FROM user_skills match_us JOIN skills match_s ON match_s.id = match_us.skill_id
			WHERE match_us.user_id = p.user_id AND match_s.name ILIKE $1))`
	var total int
	if err := s.db.QueryRow(ctx, `SELECT count(*) `+from, pattern, matchName).Scan(&total); err != nil {
		writeErr(w, r, err)
		return
	}
	rows, err := s.db.Query(ctx, `
		SELECT p.user_id, p.display_name, f.name, p.year, p.headline, card_skills.names, rating.average, rating.count
		FROM (SELECT p.user_id, p.created_at `+from+`
			ORDER BY p.created_at DESC, p.user_id LIMIT $3 OFFSET $4) listed
		JOIN profiles p ON p.user_id = listed.user_id
		JOIN faculties f ON f.id = p.faculty_id
		CROSS JOIN LATERAL (
			SELECT (array_agg(sk.name ORDER BY COALESCE(sk.name ILIKE $1, false) DESC, us.created_at, sk.name))[1:`+fmt.Sprint(cardSkillLimit)+`] AS names
			FROM user_skills us JOIN skills sk ON sk.id = us.skill_id WHERE us.user_id = p.user_id
		) card_skills
		`+ratingJoinSQL("p.user_id")+`
		ORDER BY listed.created_at DESC, listed.user_id`, pattern, matchName, req.PageSize, req.Offset())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ProfileCard, error) {
		var card ProfileCard
		err := row.Scan(&card.UserID, &card.DisplayName, &card.Faculty, &card.Year, &card.Headline, &card.Skills, &card.Rating.Average, &card.Rating.Count)
		return card, err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, newPage(req, items, total))
}
