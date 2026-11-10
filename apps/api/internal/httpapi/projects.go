package httpapi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	maxProjectName        = 120
	maxProjectDescription = 4000
	maxRoleName           = 60
	maxRoleCapacity       = 1000
	maxProjectRoles       = 50
)

func (s *Server) registerProjects() {
	s.handle("GET /projects", s.requireUser(s.listProjects))
	s.handle("POST /projects", s.requireUser(s.createProject))
	s.handle("GET /me/projects", s.requireUser(s.listMyProjects))
	s.handle("GET /projects/{id}", s.requireUser(s.getProject))
	s.handle("GET /projects/{id}/similar", s.requireUser(s.similarProjects))
	s.handle("PATCH /projects/{id}", s.requireUser(s.patchProject))
	s.handle("POST /projects/{id}/roles", s.requireUser(s.addRole))
	s.handle("PATCH /projects/{id}/roles/{roleId}", s.requireUser(s.patchRole))
	s.handle("DELETE /projects/{id}/roles/{roleId}", s.requireUser(s.deleteRole))
	s.handle("POST /projects/{id}/recruitment", s.requireUser(s.setRecruitment))
}

// Every transaction that changes who can join takes its locks in the same order:
// the project row, then the role row, then request rows. Holding to that order is
// what keeps concurrent decisions, closes and cancels from deadlocking.
const (
	// lockForRead keeps recruitment and status stable while a request is created or
	// decided. Teams can decide several requests at once, but closing waits for them.
	lockForRead = "FOR SHARE"
	// lockForWrite is for changing the project row. It isn't FOR UPDATE because that
	// would also block the foreign key check on every notification about this project.
	lockForWrite = "FOR NO KEY UPDATE"
)

var (
	errProjectNotFound = notFound("project_not_found", "We couldn't find that project.")
	errRoleNotFound    = notFound("role_not_found", "We couldn't find that role.")
	errNotLead         = forbidden("not_lead", "Only the team lead can do this.")
	errTeamEnded       = conflict("team_ended", "This team has already ended.")
	errProfileNeeded   = conflict("profile_incomplete", "Add your name, faculty and at least one skill first.")
)

type lockedProject struct {
	ID          string
	LeadUserID  string
	Status      string
	Recruitment string
	Deleted     bool
}

func (p lockedProject) ended() bool { return p.Status == "completed" || p.Status == "cancelled" }

// lockProjectRow locks the row whatever state it's in. Most callers want lockProject.
func lockProjectRow(ctx context.Context, tx pgx.Tx, id, mode string) (lockedProject, error) {
	project := lockedProject{ID: id}
	err := tx.QueryRow(ctx, `
		SELECT lead_user_id, status::text, recruitment_status::text, deleted_at IS NOT NULL
		FROM projects WHERE id = $1 `+mode, id).Scan(&project.LeadUserID, &project.Status, &project.Recruitment, &project.Deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return project, errProjectNotFound
	}
	return project, err
}

// lockProject answers 404 for a deleted team, which no longer exists to anyone.
func lockProject(ctx context.Context, tx pgx.Tx, id, mode string) (lockedProject, error) {
	project, err := lockProjectRow(ctx, tx, id, mode)
	if err == nil && project.Deleted {
		return project, errProjectNotFound
	}
	return project, err
}

// lockLedProject is lockProject plus the check that the user is the current lead.
func lockLedProject(ctx context.Context, tx pgx.Tx, id, userID, mode string) (lockedProject, error) {
	project, err := lockProject(ctx, tx, id, mode)
	if err != nil {
		return project, err
	}
	if project.LeadUserID != userID {
		return project, errNotLead
	}
	return project, nil
}

type lockedRole struct {
	ID       string
	Name     string
	Capacity int
	Taken    int
}

func (r lockedRole) full() bool { return r.Taken >= r.Capacity }

// lockRole answers 404 for a role that's removed or on another team. The count is
// its own statement on purpose: a subquery in the locking SELECT reads the snapshot
// from before the lock wait, so it would miss the member a concurrent approval just added.
func lockRole(ctx context.Context, tx pgx.Tx, projectID, roleID string) (lockedRole, error) {
	role := lockedRole{ID: roleID}
	err := tx.QueryRow(ctx, `
		SELECT name, capacity FROM project_roles
		WHERE id = $1 AND project_id = $2 AND removed_at IS NULL FOR UPDATE`, roleID, projectID).Scan(&role.Name, &role.Capacity)
	if errors.Is(err, pgx.ErrNoRows) {
		return role, errRoleNotFound
	}
	if err != nil {
		return role, err
	}
	role.Taken, err = countRoleMembers(ctx, tx, roleID)
	return role, err
}

func countRoleMembers(ctx context.Context, q querier, roleID string) (int, error) {
	var taken int
	err := q.QueryRow(ctx, `SELECT count(*) FROM project_members WHERE role_id = $1 AND left_at IS NULL`, roleID).Scan(&taken)
	return taken, err
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraint
}

type roleInput struct {
	Name     *string `json:"name"`
	Capacity *int    `json:"capacity"`
}

// normalize tidies the input and reports problems. Name and capacity are checked only when sent.
func (in *roleInput) normalize() (nameProblem, capacityProblem string) {
	if in.Name != nil {
		name := strings.Join(strings.Fields(*in.Name), " ")
		in.Name = &name
		switch {
		case name == "":
			nameProblem = "Give the role a name."
		case utf8.RuneCountInString(name) > maxRoleName:
			nameProblem = fmt.Sprintf("Keep role names to %d characters or fewer.", maxRoleName)
		}
	}
	if in.Capacity != nil && (*in.Capacity < 1 || *in.Capacity > maxRoleCapacity) {
		capacityProblem = fmt.Sprintf("Places must be from 1 to %d.", maxRoleCapacity)
	}
	return nameProblem, capacityProblem
}

type projectInput struct {
	Name          *string     `json:"name"`
	Description   *string     `json:"description"`
	CategoryID    *string     `json:"categoryId"`
	DurationValue *int        `json:"durationValue"`
	DurationUnit  *string     `json:"durationUnit"`
	Roles         []roleInput `json:"roles"`
	LeadRoleIndex *int        `json:"leadRoleIndex"`
}

// normalizeDetails checks the fields a post and an edit share. Only sent fields are checked.
func (in *projectInput) normalizeDetails() map[string]string {
	fields := map[string]string{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		in.Name = &name
		if name == "" {
			fields["name"] = "Give the project a name."
		}
		checkLength(fields, "name", in.Name, maxProjectName)
	}
	if in.Description != nil {
		description := strings.TrimSpace(*in.Description)
		in.Description = &description
		if description == "" {
			fields["description"] = "Describe the project."
		}
		checkLength(fields, "description", in.Description, maxProjectDescription)
	}
	if in.CategoryID != nil && !validUUID(*in.CategoryID) {
		fields["categoryId"] = "Choose a category from the list."
	}
	if in.DurationValue != nil && (*in.DurationValue < 1 || *in.DurationValue > math.MaxInt32) {
		fields["durationValue"] = "Expected duration must be above 0."
	}
	if in.DurationUnit != nil {
		switch *in.DurationUnit {
		case "day", "week", "month":
		default:
			fields["durationUnit"] = "Choose days, weeks or months."
		}
	}
	return fields
}

// normalizeNew adds the rules only a new post has: required fields and its roles.
func (in *projectInput) normalizeNew() map[string]string {
	fields := in.normalizeDetails()
	if in.Name == nil {
		fields["name"] = "Give the project a name."
	}
	if in.Description == nil {
		fields["description"] = "Describe the project."
	}
	if in.CategoryID == nil {
		fields["categoryId"] = "Choose a category from the list."
	}
	if in.DurationValue == nil {
		fields["durationValue"] = "Expected duration must be above 0."
	}
	if in.DurationUnit == nil {
		fields["durationUnit"] = "Choose days, weeks or months."
	}
	if problem := in.normalizeRoles(); problem != "" {
		fields["roles"] = problem
	}
	if in.LeadRoleIndex == nil || *in.LeadRoleIndex < 0 || *in.LeadRoleIndex >= len(in.Roles) {
		fields["leadRoleIndex"] = "Choose your role in the team."
	}
	return fields
}

func (in *projectInput) normalizeRoles() string {
	if len(in.Roles) == 0 {
		return "Add at least one role."
	}
	if len(in.Roles) > maxProjectRoles {
		return fmt.Sprintf("Add %d roles or fewer.", maxProjectRoles)
	}
	seen := map[string]bool{}
	places := 0
	for i := range in.Roles {
		role := &in.Roles[i]
		if role.Name == nil {
			return "Give every role a name."
		}
		if role.Capacity == nil {
			return "Give every role a number of places."
		}
		nameProblem, capacityProblem := role.normalize()
		if nameProblem != "" {
			return nameProblem
		}
		if capacityProblem != "" {
			return capacityProblem
		}
		key := strings.ToLower(*role.Name)
		if seen[key] {
			return "Each role needs a different name."
		}
		seen[key] = true
		places += *role.Capacity
	}
	if places < 2 {
		return "Leave at least one place open after your own."
	}
	return ""
}

func requireCategory(ctx context.Context, q querier, id string) error {
	var exists bool
	if err := q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM project_categories WHERE id = $1)`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return invalidFields(map[string]string{"categoryId": "Choose a category from the list."})
	}
	return nil
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request, user User) {
	var in projectInput
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, r, err)
		return
	}
	if fields := in.normalizeNew(); len(fields) > 0 {
		writeErr(w, r, invalidFields(fields))
		return
	}
	ctx := r.Context()
	var detail ProjectDetail
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if err := lockOpenAccounts(ctx, tx, user.ID); err != nil {
			return err
		}
		complete, err := isProfileComplete(ctx, tx, user.ID)
		if err != nil {
			return err
		}
		if !complete {
			return errProfileNeeded
		}
		if err := requireCategory(ctx, tx, *in.CategoryID); err != nil {
			return err
		}
		id, err := insertProject(ctx, tx, in, user.ID)
		if err != nil {
			return err
		}
		detail, err = loadProjectDetail(ctx, tx, id, user.ID)
		return err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, detail)
}

// insertProject writes the team, its roles and the lead's membership.
func insertProject(ctx context.Context, tx pgx.Tx, in projectInput, leadID string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO projects (name, description, category_id, lead_user_id, expected_duration_value, expected_duration_unit)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		*in.Name, *in.Description, *in.CategoryID, leadID, *in.DurationValue, *in.DurationUnit).Scan(&id)
	if err != nil {
		return "", err
	}
	for i, role := range in.Roles {
		var roleID string
		// created_at carries the listed order, because every row in a transaction would otherwise share one now().
		err := tx.QueryRow(ctx, `
			INSERT INTO project_roles (project_id, name, capacity, created_at)
			VALUES ($1, $2, $3, now() + $4 * interval '1 millisecond') RETURNING id`,
			id, *role.Name, *role.Capacity, i).Scan(&roleID)
		if isUniqueViolation(err, "project_roles_unique_name") {
			return "", invalidFields(map[string]string{"roles": "Each role needs a different name."})
		}
		if err != nil {
			return "", err
		}
		if i == *in.LeadRoleIndex {
			_, err := tx.Exec(ctx, `INSERT INTO project_members (project_id, role_id, user_id) VALUES ($1, $2, $3)`, id, roleID, leadID)
			if err != nil {
				return "", err
			}
		}
	}
	return id, nil
}

func (s *Server) patchProject(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	var in struct {
		Name          *string `json:"name"`
		Description   *string `json:"description"`
		CategoryID    *string `json:"categoryId"`
		DurationValue *int    `json:"durationValue"`
		DurationUnit  *string `json:"durationUnit"`
	}
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, r, err)
		return
	}
	details := projectInput{Name: in.Name, Description: in.Description, CategoryID: in.CategoryID, DurationValue: in.DurationValue, DurationUnit: in.DurationUnit}
	if fields := details.normalizeDetails(); len(fields) > 0 {
		writeErr(w, r, invalidFields(fields))
		return
	}
	ctx := r.Context()
	var detail ProjectDetail
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		project, err := lockLedProject(ctx, tx, id, user.ID, lockForWrite)
		if err != nil {
			return err
		}
		if project.ended() {
			return errTeamEnded
		}
		if details.CategoryID != nil {
			if err := requireCategory(ctx, tx, *details.CategoryID); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `
			UPDATE projects SET
				name = COALESCE($2, name),
				description = COALESCE($3, description),
				category_id = COALESCE($4::uuid, category_id),
				expected_duration_value = COALESCE($5::int, expected_duration_value),
				expected_duration_unit = COALESCE($6::duration_unit, expected_duration_unit),
				updated_at = now()
			WHERE id = $1`,
			id, details.Name, details.Description, details.CategoryID, details.DurationValue, details.DurationUnit)
		if err != nil {
			return err
		}
		detail, err = loadProjectDetail(ctx, tx, id, user.ID)
		return err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) addRole(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	var in roleInput
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, r, err)
		return
	}
	if in.Name == nil {
		empty := ""
		in.Name = &empty
	}
	if in.Capacity == nil {
		zero := 0
		in.Capacity = &zero
	}
	if err := roleFieldError(&in); err != nil {
		writeErr(w, r, err)
		return
	}
	ctx := r.Context()
	var role Role
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		project, err := lockLedProject(ctx, tx, id, user.ID, lockForRead)
		if err != nil {
			return err
		}
		if project.ended() {
			return errTeamEnded
		}
		var roleID string
		err = tx.QueryRow(ctx, `INSERT INTO project_roles (project_id, name, capacity) VALUES ($1, $2, $3) RETURNING id`,
			id, *in.Name, *in.Capacity).Scan(&roleID)
		if isUniqueViolation(err, "project_roles_unique_name") {
			return errRoleNameTaken
		}
		if err != nil {
			return err
		}
		role = newRoleView(lockedRole{ID: roleID, Name: *in.Name, Capacity: *in.Capacity})
		return nil
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, role)
}

var errRoleNameTaken = conflict("role_name_taken", "This team already has a role with that name.")

func roleFieldError(in *roleInput) error {
	nameProblem, capacityProblem := in.normalize()
	fields := map[string]string{}
	if nameProblem != "" {
		fields["name"] = nameProblem
	}
	if capacityProblem != "" {
		fields["capacity"] = capacityProblem
	}
	if len(fields) > 0 {
		return invalidFields(fields)
	}
	return nil
}

func newRoleView(role lockedRole) Role {
	return Role{ID: role.ID, Name: role.Name, Capacity: role.Capacity, Taken: role.Taken, Available: max(role.Capacity-role.Taken, 0)}
}

func (s *Server) patchRole(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	roleID, ok := pathID(r, "roleId")
	if !ok {
		writeErr(w, r, errRoleNotFound)
		return
	}
	var in roleInput
	if err := readJSON(w, r, &in); err != nil {
		writeErr(w, r, err)
		return
	}
	if err := roleFieldError(&in); err != nil {
		writeErr(w, r, err)
		return
	}
	ctx := r.Context()
	var view Role
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		project, err := lockLedProject(ctx, tx, id, user.ID, lockForRead)
		if err != nil {
			return err
		}
		if project.ended() {
			return errTeamEnded
		}
		role, err := lockRole(ctx, tx, id, roleID)
		if err != nil {
			return err
		}
		if in.Capacity != nil {
			if *in.Capacity < role.Taken {
				return conflict("capacity_below_members", "This role has more members than that. Remove a member first.")
			}
			role.Capacity = *in.Capacity
		}
		if in.Name != nil {
			role.Name = *in.Name
		}
		_, err = tx.Exec(ctx, `UPDATE project_roles SET name = $2, capacity = $3, updated_at = now() WHERE id = $1`, roleID, role.Name, role.Capacity)
		if isUniqueViolation(err, "project_roles_unique_name") {
			return errRoleNameTaken
		}
		if err != nil {
			return err
		}
		if role.full() {
			// Lowering capacity to the member count fills the role just as an approval does.
			if err := cancelPendingRequests(ctx, tx, requestScope{RoleID: roleID}, "role_full"); err != nil {
				return err
			}
		}
		view = newRoleView(role)
		return nil
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) deleteRole(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	roleID, ok := pathID(r, "roleId")
	if !ok {
		writeErr(w, r, errRoleNotFound)
		return
	}
	if err := readEmptyJSON(w, r); err != nil {
		writeErr(w, r, err)
		return
	}
	ctx := r.Context()
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		project, err := lockLedProject(ctx, tx, id, user.ID, lockForRead)
		if err != nil {
			return err
		}
		if project.ended() {
			return errTeamEnded
		}
		role, err := lockRole(ctx, tx, id, roleID)
		if err != nil {
			return err
		}
		if role.Taken > 0 {
			return conflict("role_has_members", "This role still has members. Remove them before removing the role.")
		}
		if _, err := tx.Exec(ctx, `UPDATE project_roles SET removed_at = now(), updated_at = now() WHERE id = $1`, roleID); err != nil {
			return err
		}
		return cancelPendingRequests(ctx, tx, requestScope{RoleID: roleID}, "role_removed")
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setRecruitment(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.Status != "open" && body.Status != "closed" {
		writeErr(w, r, invalidFields(map[string]string{"status": "Status must be open or closed."}))
		return
	}
	ctx := r.Context()
	var detail ProjectDetail
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		project, err := lockLedProject(ctx, tx, id, user.ID, lockForWrite)
		if err != nil {
			return err
		}
		if project.ended() {
			return errTeamEnded
		}
		if project.Recruitment != body.Status {
			_, err := tx.Exec(ctx, `UPDATE projects SET recruitment_status = $2::recruitment_status, updated_at = now() WHERE id = $1`, id, body.Status)
			if err != nil {
				return err
			}
		}
		if body.Status == "closed" {
			if err := cancelPendingRequests(ctx, tx, requestScope{ProjectID: id}, "recruitment_closed"); err != nil {
				return err
			}
		}
		detail, err = loadProjectDetail(ctx, tx, id, user.ID)
		return err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}
