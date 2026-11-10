package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const similarProjectLimit = 5

type Role struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Capacity  int    `json:"capacity"`
	Taken     int    `json:"taken"`
	Available int    `json:"available"`
}

type projectLead struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Rating      Rating `json:"rating"`
}

type ProjectCard struct {
	ID                string      `json:"id"`
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	Category          idName      `json:"category"`
	Status            string      `json:"status"`
	RecruitmentStatus string      `json:"recruitmentStatus"`
	DurationValue     int         `json:"durationValue"`
	DurationUnit      string      `json:"durationUnit"`
	PlacesTaken       int         `json:"placesTaken"`
	PlacesTotal       int         `json:"placesTotal"`
	Lead              projectLead `json:"lead"`
	Roles             []Role      `json:"roles"`
}

// Card queries expect these columns first and this FROM, so a query can add columns of its own.
// Roles that were removed count for nothing here: not in the list, not in the place totals.
const projectCardColumnsSQL = `pr.id, pr.name, pr.description, c.id, c.name, pr.status::text, pr.recruitment_status::text,
	pr.expected_duration_value, pr.expected_duration_unit::text, roles.taken, roles.total,
	u.id, ` + displayNameSQL + `, rating.average, rating.count, roles.items`

var projectCardFromSQL = `
	FROM projects pr
	JOIN project_categories c ON c.id = pr.category_id
	JOIN users u ON u.id = pr.lead_user_id
	LEFT JOIN profiles p ON p.user_id = u.id
	` + ratingJoinSQL("u.id") + `
	CROSS JOIN LATERAL (
		SELECT COALESCE(jsonb_agg(jsonb_build_object(
				'id', counted.id, 'name', counted.name, 'capacity', counted.capacity, 'taken', counted.taken,
				'available', GREATEST(counted.capacity - counted.taken, 0)
			) ORDER BY counted.created_at, counted.id), '[]'::jsonb) AS items,
			COALESCE(sum(counted.taken), 0)::int AS taken,
			COALESCE(sum(counted.capacity), 0)::int AS total
		FROM (
			SELECT r.id, r.name, r.capacity, r.created_at,
				(SELECT count(*)::int FROM project_members m WHERE m.role_id = r.id AND m.left_at IS NULL) AS taken
			FROM project_roles r WHERE r.project_id = pr.id AND r.removed_at IS NULL
		) counted
	) roles`

func cardScanTargets(card *ProjectCard) []any {
	return []any{
		&card.ID, &card.Name, &card.Description, &card.Category.ID, &card.Category.Name, &card.Status, &card.RecruitmentStatus,
		&card.DurationValue, &card.DurationUnit, &card.PlacesTaken, &card.PlacesTotal,
		&card.Lead.ID, &card.Lead.DisplayName, &card.Lead.Rating.Average, &card.Lead.Rating.Count, &card.Roles,
	}
}

// joinableSQL is one definition of a team someone can still join: open recruitment,
// not ended or deleted, and a free place in a role that wasn't removed. It expects projects as pr.
const joinableSQL = `(pr.deleted_at IS NULL AND pr.recruitment_status = 'open' AND pr.status IN ('not_started', 'in_progress')
	AND EXISTS (SELECT 1 FROM project_roles free_role
		WHERE free_role.project_id = pr.id AND free_role.removed_at IS NULL
			AND free_role.capacity > (SELECT count(*) FROM project_members m WHERE m.role_id = free_role.id AND m.left_at IS NULL)))`

func loadProjectCards(ctx context.Context, q querier, clause string, args ...any) ([]ProjectCard, error) {
	rows, err := q.Query(ctx, `SELECT `+projectCardColumnsSQL+projectCardFromSQL+` `+clause, args...)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (ProjectCard, error) {
		var card ProjectCard
		err := row.Scan(cardScanTargets(&card)...)
		return card, err
	})
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request, _ User) {
	ctx := r.Context()
	req := parsePage(r)
	var pattern, category *string
	if term := strings.TrimSpace(r.URL.Query().Get("q")); term != "" {
		like := likePattern(term)
		pattern = &like
	}
	if id := strings.TrimSpace(r.URL.Query().Get("category")); id != "" {
		// A malformed id can't match any category, so the result is an empty page.
		if !validUUID(id) {
			writeJSON(w, http.StatusOK, newPage(req, []ProjectCard{}, 0))
			return
		}
		category = &id
	}
	const where = `WHERE ` + joinableSQL + `
		AND ($1::text IS NULL OR pr.name ILIKE $1 OR pr.description ILIKE $1 OR EXISTS (
			SELECT 1 FROM project_roles match_role
			WHERE match_role.project_id = pr.id AND match_role.removed_at IS NULL AND match_role.name ILIKE $1))
		AND ($2::uuid IS NULL OR pr.category_id = $2)`
	var total int
	if err := s.db.QueryRow(ctx, `SELECT count(*) FROM projects pr `+where, pattern, category).Scan(&total); err != nil {
		writeErr(w, r, err)
		return
	}
	cards, err := loadProjectCards(ctx, s.db, where+` ORDER BY pr.created_at DESC, pr.id LIMIT $3 OFFSET $4`, pattern, category, req.PageSize, req.Offset())
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, newPage(req, cards, total))
}

func (s *Server) similarProjects(w http.ResponseWriter, r *http.Request, _ User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	ctx := r.Context()
	var category string
	err := s.db.QueryRow(ctx, `SELECT category_id FROM projects WHERE id = $1 AND deleted_at IS NULL`, id).Scan(&category)
	if errors.Is(err, pgx.ErrNoRows) {
		err = errProjectNotFound
	}
	if err != nil {
		writeErr(w, r, err)
		return
	}
	cards, err := loadProjectCards(ctx, s.db, `WHERE `+joinableSQL+` AND pr.category_id = $1 AND pr.id <> $2
		ORDER BY pr.created_at DESC, pr.id LIMIT $3`, category, id, similarProjectLimit)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]ProjectCard{"items": cards})
}

type MyProject struct {
	ProjectCard
	IsLead                 bool   `json:"isLead"`
	RoleName               string `json:"roleName"`
	RatingsOpen            bool   `json:"ratingsOpen"`
	HasPendingTransferToMe bool   `json:"hasPendingTransferToMe"`
}

func (s *Server) listMyProjects(w http.ResponseWriter, r *http.Request, user User) {
	rows, err := s.db.Query(r.Context(), `
		SELECT `+projectCardColumnsSQL+`, pr.lead_user_id = mine.user_id, mine_role.name,
			pr.status = 'completed' AND pr.completed_at + `+ratingWindowSQL+` > now(),
			EXISTS (SELECT 1 FROM leadership_transfers t WHERE t.project_id = pr.id AND t.to_user_id = mine.user_id AND t.status = 'pending')
		`+projectCardFromSQL+`
		JOIN project_members mine ON mine.project_id = pr.id AND mine.user_id = $1 AND mine.left_at IS NULL
		JOIN project_roles mine_role ON mine_role.id = mine.role_id
		WHERE pr.deleted_at IS NULL
		ORDER BY mine.joined_at DESC, pr.id`, user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (MyProject, error) {
		var item MyProject
		targets := append(cardScanTargets(&item.ProjectCard), &item.IsLead, &item.RoleName, &item.RatingsOpen, &item.HasPendingTransferToMe)
		err := row.Scan(targets...)
		return item, err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]MyProject{"items": items})
}

type Member struct {
	UserID      string    `json:"userId"`
	DisplayName string    `json:"displayName"`
	Faculty     *string   `json:"faculty"`
	RoleID      string    `json:"roleId"`
	RoleName    string    `json:"roleName"`
	JoinedAt    time.Time `json:"joinedAt"`
	IsLead      bool      `json:"isLead"`
}

type projectContact struct {
	LineURL    *string `json:"lineUrl"`
	DiscordURL *string `json:"discordUrl"`
}

type pendingTransfer struct {
	ID            string `json:"id"`
	ToUserID      string `json:"toUserId"`
	ToDisplayName string `json:"toDisplayName"`
	FromUserID    string `json:"fromUserId"`
}

type ratingWindow struct {
	ClosesAt time.Time `json:"closesAt"`
	Open     bool      `json:"open"`
}

type applyBlock struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type projectViewer struct {
	IsLead         bool        `json:"isLead"`
	IsMember       bool        `json:"isMember"`
	PendingRequest *Request    `json:"pendingRequest"`
	ApplyBlock     *applyBlock `json:"applyBlock"`
}

type ProjectDetail struct {
	ProjectCard
	CreatedAt       time.Time        `json:"createdAt"`
	StartedAt       *time.Time       `json:"startedAt"`
	CompletedAt     *time.Time       `json:"completedAt"`
	CancelledAt     *time.Time       `json:"cancelledAt"`
	ExpectedEndDate *string          `json:"expectedEndDate"`
	Members         []Member         `json:"members"`
	Contact         *projectContact  `json:"contact"`
	PendingTransfer *pendingTransfer `json:"pendingTransfer"`
	RatingWindow    *ratingWindow    `json:"ratingWindow"`
	EndDateReached  bool             `json:"endDateReached"`
	Viewer          projectViewer    `json:"viewer"`
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request, user User) {
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errProjectNotFound)
		return
	}
	detail, err := loadProjectDetail(r.Context(), s.db, id, user.ID)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// loadProjectDetail reads the team as viewerID may see it, with 404 for a deleted one.
func loadProjectDetail(ctx context.Context, q querier, id, viewerID string) (ProjectDetail, error) {
	var detail ProjectDetail
	cards, err := loadProjectCards(ctx, q, `WHERE pr.id = $1 AND pr.deleted_at IS NULL`, id)
	if err != nil {
		return detail, err
	}
	if len(cards) == 0 {
		return detail, errProjectNotFound
	}
	detail.ProjectCard = cards[0]
	detail.Viewer.IsLead = detail.Lead.ID == viewerID

	var contact projectContact
	var closesAt *time.Time
	var ratingsOpen, endDateReached bool
	err = q.QueryRow(ctx, `
		SELECT created_at, started_at, completed_at, cancelled_at, to_char(expected_end_date, 'YYYY-MM-DD'), line_url, discord_url,
			completed_at + `+ratingWindowSQL+`, COALESCE(completed_at + `+ratingWindowSQL+` > now(), false),
			COALESCE(status = 'in_progress' AND expected_end_date <= `+bangkokTodaySQL+`, false)
		FROM projects WHERE id = $1`, id).Scan(
		&detail.CreatedAt, &detail.StartedAt, &detail.CompletedAt, &detail.CancelledAt, &detail.ExpectedEndDate,
		&contact.LineURL, &contact.DiscordURL, &closesAt, &ratingsOpen, &endDateReached)
	if err != nil {
		return detail, err
	}
	if closesAt != nil {
		detail.RatingWindow = &ratingWindow{ClosesAt: *closesAt, Open: ratingsOpen}
	}
	detail.EndDateReached = endDateReached && detail.Viewer.IsLead

	if detail.Members, err = loadMembers(ctx, q, id, detail.Lead.ID); err != nil {
		return detail, err
	}
	for _, member := range detail.Members {
		if member.UserID == viewerID {
			detail.Viewer.IsMember = true
		}
	}
	// The one place contact links leave the server, and only for a current member.
	if detail.Viewer.IsMember {
		detail.Contact = &contact
	}
	if detail.PendingTransfer, err = loadPendingTransfer(ctx, q, id, viewerID, detail.Viewer.IsLead); err != nil {
		return detail, err
	}
	pending, err := loadRequests(ctx, q, viewerID, `WHERE jr.project_id = $1 AND jr.student_id = $2 AND jr.status = 'pending' AND jr.expires_at > now()`, id, viewerID)
	if err != nil {
		return detail, err
	}
	if len(pending) > 0 {
		detail.Viewer.PendingRequest = &pending[0]
	}
	detail.Viewer.ApplyBlock, err = loadApplyBlock(ctx, q, detail, viewerID)
	return detail, err
}

func loadMembers(ctx context.Context, q querier, projectID, leadID string) ([]Member, error) {
	rows, err := q.Query(ctx, `
		SELECT m.user_id, `+displayNameSQL+`, CASE WHEN u.closed_at IS NULL THEN f.name END, m.role_id, role.name, m.joined_at, m.user_id = $2
		FROM project_members m
		JOIN project_roles role ON role.id = m.role_id
		JOIN users u ON u.id = m.user_id
		LEFT JOIN profiles p ON p.user_id = u.id
		LEFT JOIN faculties f ON f.id = p.faculty_id
		WHERE m.project_id = $1 AND m.left_at IS NULL
		ORDER BY m.user_id = $2 DESC, m.joined_at, m.id`, projectID, leadID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Member, error) {
		var member Member
		err := row.Scan(&member.UserID, &member.DisplayName, &member.Faculty, &member.RoleID, &member.RoleName, &member.JoinedAt, &member.IsLead)
		return member, err
	})
}

// loadPendingTransfer returns the team's pending transfer for its lead and recipient, and nil for anyone else.
func loadPendingTransfer(ctx context.Context, q querier, projectID, viewerID string, viewerIsLead bool) (*pendingTransfer, error) {
	var transfer pendingTransfer
	err := q.QueryRow(ctx, `
		SELECT t.id, t.to_user_id, `+displayNameSQL+`, t.from_user_id
		FROM leadership_transfers t
		JOIN users u ON u.id = t.to_user_id
		LEFT JOIN profiles p ON p.user_id = u.id
		WHERE t.project_id = $1 AND t.status = 'pending'`, projectID).Scan(&transfer.ID, &transfer.ToUserID, &transfer.ToDisplayName, &transfer.FromUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !viewerIsLead && transfer.ToUserID != viewerID {
		return nil, nil
	}
	return &transfer, nil
}

// loadApplyBlock is the team-wide reason this viewer can't apply, most relevant first.
// It says what creating an application would answer, ahead of the attempt.
func loadApplyBlock(ctx context.Context, q querier, detail ProjectDetail, viewerID string) (*applyBlock, error) {
	switch {
	case detail.Viewer.IsMember:
		return &applyBlock{"lead_or_member", "You're already on this team."}, nil
	case detail.Status == "completed" || detail.Status == "cancelled":
		return &applyBlock{"team_ended", "This team has already ended."}, nil
	case detail.RecruitmentStatus != "open":
		return &applyBlock{"recruitment_closed", "This team isn't recruiting right now."}, nil
	case detail.Viewer.PendingRequest != nil:
		return &applyBlock{"pending_request", "You already have a pending request with this team."}, nil
	}
	latestType, latestStatus, err := latestRequest(ctx, q, detail.ID, viewerID)
	if err != nil {
		return nil, err
	}
	if latestType == "application" && latestStatus == "rejected" {
		return &applyBlock{"previously_rejected", "The lead turned down your application, so they need to invite you to try again."}, nil
	}
	complete, err := isProfileComplete(ctx, q, viewerID)
	if err != nil {
		return nil, err
	}
	if !complete {
		return &applyBlock{"profile_incomplete", "Add your name, faculty and at least one skill before applying."}, nil
	}
	return nil, nil
}
