package httpapi

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/thinc-org/CU-Profile/apps/api/internal/seed"
)

const (
	danEmail = "6730000321@student.chula.ac.th"
	eveEmail = "6730000421@student.chula.ac.th"
)

type requestJSON struct {
	ID           string  `json:"id"`
	Type         string  `json:"type"`
	Status       string  `json:"status"`
	CancelReason *string `json:"cancelReason"`
	ExpiresAt    string  `json:"expiresAt"`
	Student      struct {
		ID string `json:"id"`
	} `json:"student"`
	Viewer map[string]bool `json:"viewer"`
}

type roleJSON struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Capacity  int    `json:"capacity"`
	Taken     int    `json:"taken"`
	Available int    `json:"available"`
}

type projectJSON struct {
	ID          string     `json:"id"`
	PlacesTaken int        `json:"placesTaken"`
	PlacesTotal int        `json:"placesTotal"`
	Roles       []roleJSON `json:"roles"`
	Contact     *struct {
		LineURL string `json:"lineUrl"`
	} `json:"contact"`
	Members []struct {
		UserID string `json:"userId"`
		IsLead bool   `json:"isLead"`
	} `json:"members"`
	Viewer struct {
		IsLead         bool         `json:"isLead"`
		IsMember       bool         `json:"isMember"`
		PendingRequest *requestJSON `json:"pendingRequest"`
		ApplyBlock     *struct {
			Code string `json:"code"`
		} `json:"applyBlock"`
	} `json:"viewer"`
}

func (r projectJSON) role(t *testing.T, name string) roleJSON {
	t.Helper()
	for _, role := range r.Roles {
		if role.Name == name {
			return role
		}
	}
	t.Fatalf("no role %q in %+v", name, r.Roles)
	return roleJSON{}
}

// student signs someone in with a complete profile.
func (a *testAPI) student(email, name string) *testClient {
	a.t.Helper()
	c := a.signIn(email)
	c.completeProfile(name, "Go")
	return c
}

func (a *testAPI) categoryID() string {
	a.t.Helper()
	var id string
	a.scalar(&id, `SELECT id FROM project_categories ORDER BY name LIMIT 1`)
	return id
}

// postProject creates a team through the API. The lead holds the first role, "Lead",
// and each extra role has the capacity given.
func (a *testAPI) postProject(lead *testClient, name string, capacities map[string]int) projectJSON {
	a.t.Helper()
	roles := []map[string]any{{"name": "Lead", "capacity": 1}}
	for role, capacity := range capacities {
		roles = append(roles, map[string]any{"name": role, "capacity": capacity})
	}
	var project projectJSON
	mustDecode(a.t, lead.expect(http.StatusCreated, "POST", "/projects", map[string]any{
		"name": name, "description": "A description", "categoryId": a.categoryID(),
		"durationValue": 7, "durationUnit": "week", "roles": roles, "leadRoleIndex": 0,
	}), &project)
	return project
}

func (c *testClient) applyTo(project projectJSON, role string) requestJSON {
	c.api.t.Helper()
	var request requestJSON
	mustDecode(c.api.t, c.expect(http.StatusCreated, "POST", "/projects/"+project.ID+"/applications",
		map[string]any{"roleId": project.role(c.api.t, role).ID}), &request)
	return request
}

func (c *testClient) inviteTo(project projectJSON, role string, student *testClient) requestJSON {
	c.api.t.Helper()
	var request requestJSON
	mustDecode(c.api.t, c.expect(http.StatusCreated, "POST", "/projects/"+project.ID+"/invitations",
		map[string]any{"roleId": project.role(c.api.t, role).ID, "studentId": student.userID}), &request)
	return request
}

func (c *testClient) decide(request requestJSON, action string) requestJSON {
	c.api.t.Helper()
	var out requestJSON
	mustDecode(c.api.t, c.expect(http.StatusOK, "POST", "/requests/"+request.ID+"/"+action, map[string]any{}), &out)
	return out
}

// expectCode sends a request and checks both the status and the error code.
func (c *testClient) expectCode(status int, code, method, path string, body any) {
	c.api.t.Helper()
	if got := errorCode(c.api.t, c.expect(status, method, path, body)); got != code {
		c.api.t.Fatalf("%s %s: code %q, want %q", method, path, got, code)
	}
}

func (a *testAPI) requestState(id string) (status, reason string) {
	a.t.Helper()
	a.scalar(&status, `SELECT status::text FROM join_requests WHERE id = $1`, id)
	a.scalar(&reason, `SELECT COALESCE(cancel_reason::text, '') FROM join_requests WHERE id = $1`, id)
	return status, reason
}

func (a *testAPI) notificationCount(user *testClient, notificationType string) int {
	a.t.Helper()
	var count int
	a.scalar(&count, `SELECT count(*) FROM notifications WHERE user_id = $1 AND type = $2`, user.userID, notificationType)
	return count
}

// race runs every call at the same moment and returns the statuses and error codes in order.
func race(t *testing.T, calls ...func() (*http.Response, []byte)) (statuses []int, bodies [][]byte) {
	t.Helper()
	statuses, bodies = make([]int, len(calls)), make([][]byte, len(calls))
	var ready, done sync.WaitGroup
	start := make(chan struct{})
	for i, call := range calls {
		ready.Add(1)
		done.Add(1)
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			response, body := call()
			statuses[i], bodies[i] = response.StatusCode, body
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
	return statuses, bodies
}

// rawCall can run off the test goroutine, unlike send, which may call t.Fatal.
func (c *testClient) rawCall(method, path, body string) func() (*http.Response, []byte) {
	return func() (*http.Response, []byte) {
		request, err := http.NewRequest(method, c.api.server.URL+apiPrefix+path, strings.NewReader(body))
		if err != nil {
			return &http.Response{}, []byte(err.Error())
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := c.http.Do(request)
		if err != nil {
			return &http.Response{}, []byte(err.Error())
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			return &http.Response{}, []byte(err.Error())
		}
		return response, raw
	}
}

func checkOneWinner(t *testing.T, statuses []int, bodies [][]byte) (winner, loser int) {
	t.Helper()
	switch {
	case statuses[0] == http.StatusOK && statuses[1] == http.StatusConflict:
		winner, loser = 0, 1
	case statuses[1] == http.StatusOK && statuses[0] == http.StatusConflict:
		winner, loser = 1, 0
	default:
		t.Fatalf("want one 200 and one 409, got %v: %s / %s", statuses, bodies[0], bodies[1])
	}
	if code := errorCode(t, bodies[loser]); code != "role_full" {
		t.Fatalf("loser got %q: %s", code, bodies[loser])
	}
	return winner, loser
}

func TestLastPlaceRaceHasOneWinner(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	cara := api.student(caraEmail, "Cara")

	for round := range 8 {
		project := api.postProject(lead, "Race", map[string]int{"Dev": 1})
		first, second := bob.applyTo(project, "Dev"), cara.applyTo(project, "Dev")
		statuses, bodies := race(t,
			lead.rawCall("POST", "/requests/"+first.ID+"/approve", "{}"),
			lead.rawCall("POST", "/requests/"+second.ID+"/approve", "{}"),
		)
		_, loser := checkOneWinner(t, statuses, bodies)

		var members int
		api.scalar(&members, `SELECT count(*) FROM project_members WHERE role_id = $1 AND left_at IS NULL`, project.role(t, "Dev").ID)
		if members != 1 {
			t.Fatalf("round %d: %d members hold one place", round, members)
		}
		loserRequest := []requestJSON{first, second}[loser]
		if status, reason := api.requestState(loserRequest.ID); status != "cancelled" || reason != "role_full" {
			t.Fatalf("round %d: loser is %s/%s", round, status, reason)
		}
	}
	// One cancellation per round, each told to whichever student lost it.
	if total := api.notificationCount(bob, notifRequestCancelled) + api.notificationCount(cara, notifRequestCancelled); total != 8 {
		t.Fatalf("%d role_full notifications, want 8", total)
	}
}

func TestAcceptAndApproveRaceForLastPlace(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	cara := api.student(caraEmail, "Cara")
	for round := range 8 {
		project := api.postProject(lead, "Race", map[string]int{"Dev": 1})
		invitation := lead.inviteTo(project, "Dev", bob)
		application := cara.applyTo(project, "Dev")
		statuses, bodies := race(t,
			bob.rawCall("POST", "/requests/"+invitation.ID+"/accept", "{}"),
			lead.rawCall("POST", "/requests/"+application.ID+"/approve", "{}"),
		)
		checkOneWinner(t, statuses, bodies)
		var members int
		api.scalar(&members, `SELECT count(*) FROM project_members WHERE role_id = $1 AND left_at IS NULL`, project.role(t, "Dev").ID)
		if members != 1 {
			t.Fatalf("round %d: %d members hold one place", round, members)
		}
	}
}

// Closing recruitment while an approval is in flight must leave a consistent result,
// whichever gets the project lock first, and must not deadlock.
func TestCloseRecruitmentRacesWithApproval(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	for round := range 8 {
		project := api.postProject(lead, "Race", map[string]int{"Dev": 1})
		application := bob.applyTo(project, "Dev")
		statuses, _ := race(t,
			lead.rawCall("POST", "/requests/"+application.ID+"/approve", "{}"),
			lead.rawCall("POST", "/projects/"+project.ID+"/recruitment", `{"status":"closed"}`),
		)
		if statuses[1] != http.StatusOK {
			t.Fatalf("round %d: close answered %d", round, statuses[1])
		}
		status, reason := api.requestState(application.ID)
		var members int
		api.scalar(&members, `SELECT count(*) FROM project_members WHERE role_id = $1 AND left_at IS NULL`, project.role(t, "Dev").ID)
		approved := status == "approved" && members == 1 && statuses[0] == http.StatusOK
		cancelled := status == "cancelled" && reason == "recruitment_closed" && members == 0 && statuses[0] == http.StatusConflict
		if !approved && !cancelled {
			t.Fatalf("round %d: approve %d, request %s/%s, %d members", round, statuses[0], status, reason, members)
		}
	}
}

func TestProjectMutationsRequireTheLead(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	member := api.student(bobEmail, "Member")
	outsider := api.student(caraEmail, "Outsider")
	target := api.student(danEmail, "Target")
	project := api.postProject(lead, "Team", map[string]int{"Dev": 3, "Spare": 1})
	bobApp := member.applyTo(project, "Dev")
	lead.decide(bobApp, "approve")
	dev, spare := project.role(t, "Dev").ID, project.role(t, "Spare").ID
	base := "/projects/" + project.ID

	calls := []struct {
		method, path string
		body         any
	}{
		{"PATCH", base, map[string]any{"name": "Taken over"}},
		{"POST", base + "/roles", map[string]any{"name": "New", "capacity": 1}},
		{"PATCH", base + "/roles/" + dev, map[string]any{"capacity": 9}},
		{"DELETE", base + "/roles/" + spare, nil},
		{"POST", base + "/recruitment", map[string]any{"status": "closed"}},
		{"POST", base + "/invitations", map[string]any{"roleId": dev, "studentId": target.userID}},
		{"GET", base + "/requests", nil},
	}
	for _, call := range calls {
		for _, who := range []*testClient{member, outsider} {
			who.expectCode(http.StatusForbidden, "not_lead", call.method, call.path, call.body)
		}
	}
	anonymous := api.anonymous()
	for _, call := range calls {
		anonymous.expect(http.StatusUnauthorized, call.method, call.path, call.body)
	}
	var name, recruitment string
	api.scalar(&name, `SELECT name FROM projects WHERE id = $1`, project.ID)
	api.scalar(&recruitment, `SELECT recruitment_status::text FROM projects WHERE id = $1`, project.ID)
	var removed, invites int
	api.scalar(&removed, `SELECT count(*) FROM project_roles WHERE project_id = $1 AND removed_at IS NOT NULL`, project.ID)
	api.scalar(&invites, `SELECT count(*) FROM join_requests WHERE project_id = $1 AND type = 'invitation'`, project.ID)
	if name != "Team" || recruitment != "open" || removed != 0 || invites != 0 {
		t.Fatalf("a refused call changed something: %s %s %d %d", name, recruitment, removed, invites)
	}
}

func TestRequestDecisionsCheckTheActor(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	member := api.student(caraEmail, "Member")
	outsider := api.student(danEmail, "Outsider")
	invited := api.student(eveEmail, "Invited")
	project := api.postProject(lead, "Team", map[string]int{"Dev": 5})
	lead.decide(member.applyTo(project, "Dev"), "approve")

	application := bob.applyTo(project, "Dev")
	invitation := lead.inviteTo(project, "Dev", invited)
	appPath, invPath := "/requests/"+application.ID, "/requests/"+invitation.ID

	// People with no part in a request can't learn it exists, even teammates.
	for _, who := range []*testClient{outsider, member} {
		who.expectCode(http.StatusNotFound, "request_not_found", "GET", appPath, nil)
		for _, action := range []string{"approve", "reject", "withdraw"} {
			who.expectCode(http.StatusNotFound, "request_not_found", "POST", appPath+"/"+action, map[string]any{})
		}
		for _, action := range []string{"accept", "decline", "cancel"} {
			who.expectCode(http.StatusNotFound, "request_not_found", "POST", invPath+"/"+action, map[string]any{})
		}
	}
	// Each side can only do its own part.
	bob.expectCode(http.StatusForbidden, "not_allowed", "POST", appPath+"/approve", map[string]any{})
	bob.expectCode(http.StatusForbidden, "not_allowed", "POST", appPath+"/reject", map[string]any{})
	lead.expectCode(http.StatusForbidden, "not_allowed", "POST", appPath+"/withdraw", map[string]any{})
	invited.expectCode(http.StatusForbidden, "not_allowed", "POST", invPath+"/cancel", map[string]any{})
	lead.expectCode(http.StatusForbidden, "not_allowed", "POST", invPath+"/accept", map[string]any{})
	lead.expectCode(http.StatusForbidden, "not_allowed", "POST", invPath+"/decline", map[string]any{})
	// The action has to fit the kind of request.
	lead.expectCode(http.StatusConflict, "wrong_request_type", "POST", invPath+"/approve", map[string]any{})
	bob.expectCode(http.StatusConflict, "wrong_request_type", "POST", appPath+"/accept", map[string]any{})

	if status, _ := api.requestState(application.ID); status != "pending" {
		t.Fatalf("application is %s", status)
	}
	if status, _ := api.requestState(invitation.ID); status != "pending" {
		t.Fatalf("invitation is %s", status)
	}
	// viewer flags agree with what the server allows.
	var seen requestJSON
	mustDecode(t, lead.expect(http.StatusOK, "GET", appPath, nil), &seen)
	if !seen.Viewer["canApprove"] || !seen.Viewer["canReject"] || seen.Viewer["canWithdraw"] || !seen.Viewer["isLead"] {
		t.Fatalf("lead viewer: %+v", seen.Viewer)
	}
	mustDecode(t, invited.expect(http.StatusOK, "GET", invPath, nil), &seen)
	if !seen.Viewer["canAccept"] || !seen.Viewer["canDecline"] || seen.Viewer["canCancel"] || !seen.Viewer["isStudent"] {
		t.Fatalf("student viewer: %+v", seen.Viewer)
	}
	// Every outcome tells the other side.
	lead.decide(application, "approve")
	if api.notificationCount(bob, notifApplicationApproved) != 1 || api.notificationCount(lead, notifApplicationReceived) < 1 {
		t.Fatal("approval not notified")
	}
	invited.decide(invitation, "decline")
	if api.notificationCount(lead, notifInvitationDeclined) != 1 || api.notificationCount(invited, notifInvitationReceived) != 1 {
		t.Fatal("decline not notified")
	}
	lead.expectCode(http.StatusConflict, "request_not_pending", "POST", appPath+"/reject", map[string]any{})
}

func TestRetryRuleBothDirections(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	project := api.postProject(lead, "Team", map[string]int{"Dev": 5})
	dev := project.role(t, "Dev").ID
	apply := map[string]any{"roleId": dev}
	invite := map[string]any{"roleId": dev, "studentId": bob.userID}
	applications, invitations := "/projects/"+project.ID+"/applications", "/projects/"+project.ID+"/invitations"

	// A rejected student can't apply again, but the lead can invite.
	lead.decide(bob.applyTo(project, "Dev"), "reject")
	bob.expectCode(http.StatusConflict, "previously_rejected", "POST", applications, apply)
	var detail projectJSON
	mustDecode(t, bob.expect(http.StatusOK, "GET", "/projects/"+project.ID, nil), &detail)
	if detail.Viewer.ApplyBlock == nil || detail.Viewer.ApplyBlock.Code != "previously_rejected" {
		t.Fatalf("apply block: %+v", detail.Viewer.ApplyBlock)
	}
	invitation := lead.inviteTo(project, "Dev", bob)

	// A declined invitation stops the lead, but the student can apply, which starts over.
	bob.decide(invitation, "decline")
	lead.expectCode(http.StatusConflict, "previously_declined", "POST", invitations, invite)
	application := bob.applyTo(project, "Dev")

	// Withdrawing or cancelling leaves both sides free.
	bob.decide(application, "withdraw")
	again := lead.inviteTo(project, "Dev", bob)
	lead.decide(again, "cancel")
	if status, reason := api.requestState(again.ID); status != "cancelled" || reason != "lead_cancelled" {
		t.Fatalf("cancelled invitation is %s/%s", status, reason)
	}
	bob.decide(bob.applyTo(project, "Dev"), "withdraw")
	mustDecode(t, bob.expect(http.StatusOK, "GET", "/projects/"+project.ID, nil), &detail)
	if detail.Viewer.ApplyBlock != nil {
		t.Fatalf("apply block after withdrawing: %+v", detail.Viewer.ApplyBlock)
	}
}

func TestOnePendingPerStudentPerTeamAcrossTypes(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	cara := api.student(caraEmail, "Cara")
	project := api.postProject(lead, "Team", map[string]int{"Dev": 5, "Ops": 5})
	dev, ops := project.role(t, "Dev").ID, project.role(t, "Ops").ID
	applications, invitations := "/projects/"+project.ID+"/applications", "/projects/"+project.ID+"/invitations"

	// Even a different role counts: the rule is per team.
	bob.applyTo(project, "Dev")
	bob.expectCode(http.StatusConflict, "pending_request", "POST", applications, map[string]any{"roleId": ops})
	lead.expectCode(http.StatusConflict, "pending_request", "POST", invitations, map[string]any{"roleId": ops, "studentId": bob.userID})

	lead.inviteTo(project, "Ops", cara)
	cara.expectCode(http.StatusConflict, "pending_request", "POST", applications, map[string]any{"roleId": dev})
	lead.expectCode(http.StatusConflict, "pending_request", "POST", invitations, map[string]any{"roleId": dev, "studentId": cara.userID})

	var detail projectJSON
	mustDecode(t, cara.expect(http.StatusOK, "GET", "/projects/"+project.ID, nil), &detail)
	if detail.Viewer.PendingRequest == nil || detail.Viewer.ApplyBlock == nil || detail.Viewer.ApplyBlock.Code != "pending_request" {
		t.Fatalf("viewer: %+v", detail.Viewer)
	}
}

func TestApplyingAndInvitingGuards(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	incomplete := api.signIn(caraEmail)
	closed := api.student(danEmail, "Closed")
	closed.expect(http.StatusNoContent, "POST", "/me/close", map[string]any{})
	project := api.postProject(lead, "Team", map[string]int{"Dev": 2, "Solo": 1})
	dev, solo := project.role(t, "Dev").ID, project.role(t, "Solo").ID
	applications, invitations := "/projects/"+project.ID+"/applications", "/projects/"+project.ID+"/invitations"

	// The actor's profile has to be complete.
	incomplete.expectCode(http.StatusConflict, "profile_incomplete", "POST", applications, map[string]any{"roleId": dev})
	incomplete.expectCode(http.StatusConflict, "profile_incomplete", "POST", "/projects", map[string]any{
		"name": "N", "description": "D", "categoryId": api.categoryID(), "durationValue": 1, "durationUnit": "day",
		"roles": []map[string]any{{"name": "A", "capacity": 2}}, "leadRoleIndex": 0,
	})
	api.exec(`DELETE FROM user_skills WHERE user_id = $1`, lead.userID)
	lead.expectCode(http.StatusConflict, "profile_incomplete", "POST", invitations, map[string]any{"roleId": dev, "studentId": bob.userID})
	lead.completeProfile("Lead", "Go")

	// Invitees must be someone who can be invited, and not on the team.
	lead.expectCode(http.StatusNotFound, "student_not_found", "POST", invitations, map[string]any{"roleId": dev, "studentId": closed.userID})
	lead.expectCode(http.StatusNotFound, "student_not_found", "POST", invitations, map[string]any{"roleId": dev, "studentId": incomplete.userID})
	lead.expectCode(http.StatusConflict, "already_member", "POST", invitations, map[string]any{"roleId": dev, "studentId": lead.userID})
	lead.expectCode(http.StatusConflict, "already_member", "POST", applications, map[string]any{"roleId": dev})
	lead.decide(bob.applyTo(project, "Dev"), "approve")
	lead.expectCode(http.StatusConflict, "already_member", "POST", invitations, map[string]any{"roleId": dev, "studentId": bob.userID})
	bob.expectCode(http.StatusConflict, "already_member", "POST", applications, map[string]any{"roleId": dev})

	// Roles have to be real, free and on this team.
	other := api.postProject(lead, "Other", map[string]int{"Elsewhere": 2})
	cara := api.student(eveEmail, "Eve")
	cara.expectCode(http.StatusNotFound, "role_not_found", "POST", applications, map[string]any{"roleId": other.role(t, "Elsewhere").ID})
	cara.expect(http.StatusBadRequest, "POST", applications, map[string]any{})

	api.moveMember(project.ID, solo, bob.userID)
	cara.expectCode(http.StatusConflict, "role_full", "POST", applications, map[string]any{"roleId": solo})
	lead.expectCode(http.StatusConflict, "role_full", "POST", invitations, map[string]any{"roleId": solo, "studentId": cara.userID})

	api.exec(`UPDATE projects SET recruitment_status = 'closed' WHERE id = $1`, project.ID)
	cara.expectCode(http.StatusConflict, "recruitment_closed", "POST", applications, map[string]any{"roleId": dev})
	api.exec(`UPDATE projects SET status = 'cancelled', started_at = now(), cancelled_at = now(), expected_end_date = now() WHERE id = $1`, project.ID)
	cara.expectCode(http.StatusConflict, "team_ended", "POST", applications, map[string]any{"roleId": dev})
	lead.expectCode(http.StatusConflict, "team_ended", "POST", "/projects/"+project.ID+"/recruitment", map[string]any{"status": "open"})
	lead.expectCode(http.StatusConflict, "team_ended", "PATCH", "/projects/"+project.ID, map[string]any{"name": "Late"})
}

// moveMember puts someone in a role directly, ending any place they hold on the team.
func (a *testAPI) moveMember(projectID, roleID, userID string) {
	a.t.Helper()
	a.exec(`UPDATE project_members SET left_at = now(), left_reason = 'left' WHERE project_id = $1 AND user_id = $2 AND left_at IS NULL`, projectID, userID)
	a.exec(`INSERT INTO project_members (project_id, role_id, user_id) VALUES ($1, $2, $3)`, projectID, roleID, userID)
}

func TestExpiryBlocksDecisionsAndAllowsANewRequest(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	cara := api.student(caraEmail, "Cara")
	project := api.postProject(lead, "Team", map[string]int{"Dev": 5})

	application := bob.applyTo(project, "Dev")
	invitation := lead.inviteTo(project, "Dev", cara)
	var hours float64
	api.scalar(&hours, `SELECT extract(epoch FROM expires_at - created_at) / 3600 FROM join_requests WHERE id = $1`, application.ID)
	if hours != 48 {
		t.Fatalf("deadline is %v hours after creation", hours)
	}
	if application.ExpiresAt == "" {
		t.Fatal("expiresAt missing")
	}

	api.exec(`UPDATE join_requests SET expires_at = now() - interval '1 minute' WHERE project_id = $1`, project.ID)
	lead.expectCode(http.StatusConflict, "request_expired", "POST", "/requests/"+application.ID+"/approve", map[string]any{})
	cara.expectCode(http.StatusConflict, "request_expired", "POST", "/requests/"+invitation.ID+"/accept", map[string]any{})
	for _, id := range []string{application.ID, invitation.ID} {
		if status, _ := api.requestState(id); status != "expired" {
			t.Fatalf("request is %s", status)
		}
	}
	var members int
	api.scalar(&members, `SELECT count(*) FROM project_members WHERE project_id = $1 AND left_at IS NULL`, project.ID)
	if members != 1 {
		t.Fatalf("%d members after expired decisions", members)
	}
	// Both sides hear about each request exactly once, however many sweeps ran.
	for user, want := range map[*testClient]int{lead: 2, bob: 1, cara: 1} {
		if got := api.notificationCount(user, notifRequestExpired); got != want {
			t.Fatalf("%s has %d expiry notifications, want %d", user.email, got, want)
		}
	}
	lead.expect(http.StatusOK, "GET", "/notifications", nil)
	if got := api.notificationCount(lead, notifRequestExpired); got != 2 {
		t.Fatalf("a second sweep notified again: %d", got)
	}
	// Other decisions answer the same way.
	lead.expectCode(http.StatusConflict, "request_expired", "POST", "/requests/"+application.ID+"/reject", map[string]any{})
	bob.expectCode(http.StatusConflict, "request_expired", "POST", "/requests/"+application.ID+"/withdraw", map[string]any{})

	// Expiry frees the student and the lead to try again right away.
	bob.applyTo(project, "Dev")
	lead.inviteTo(project, "Dev", cara)
}

func TestExpiredRequestsStopCountingAsPendingBeforeTheSweep(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	project := api.postProject(lead, "Team", map[string]int{"Dev": 5})
	application := bob.applyTo(project, "Dev")
	api.exec(`UPDATE join_requests SET expires_at = now() - interval '1 minute' WHERE id = $1`, application.ID)

	// GET /projects/{id} doesn't run the sweep, so the row is still pending here.
	var detail projectJSON
	mustDecode(t, bob.expect(http.StatusOK, "GET", "/projects/"+project.ID, nil), &detail)
	if status, _ := api.requestState(application.ID); status != "pending" {
		t.Fatalf("the sweep ran early: %s", status)
	}
	if detail.Viewer.PendingRequest != nil || detail.Viewer.ApplyBlock != nil {
		t.Fatalf("past-due request still counts: %+v", detail.Viewer)
	}
	// The read itself reports it as expired, with nothing left to act on.
	requests, err := loadRequestFor(context.Background(), api.pool, application.ID, lead.userID)
	if err != nil {
		t.Fatal(err)
	}
	if requests.Status != "expired" || requests.ResolvedAt == nil || requests.Viewer.CanApprove || requests.Viewer.CanReject {
		t.Fatalf("read as %+v", requests)
	}
	// Reading through the API sweeps, and notifies.
	lead.expect(http.StatusOK, "GET", "/me/requests", nil)
	if status, _ := api.requestState(application.ID); status != "expired" {
		t.Fatalf("after reading requests: %s", status)
	}
	if api.notificationCount(lead, notifRequestExpired) != 1 || api.notificationCount(bob, notifRequestExpired) != 1 {
		t.Fatal("expiry did not notify both sides")
	}
}

// A deadline can pass between the sweep and the decision, so the decision has to catch it itself.
func TestDecisionCatchesADeadlineTheSweepMissed(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	project := api.postProject(lead, "Team", map[string]int{"Dev": 5})
	application := bob.applyTo(project, "Dev")
	api.exec(`UPDATE join_requests SET expires_at = now() - interval '1 minute' WHERE id = $1`, application.ID)

	ctx := context.Background()
	var expiredNow bool
	err := pgx.BeginFunc(ctx, api.pool, func(tx pgx.Tx) error {
		var err error
		_, expiredNow, err = decideTx(ctx, tx, application.ID, User{ID: lead.userID}, approveDecision)
		return err
	})
	if err != nil || !expiredNow {
		t.Fatalf("expiredNow %v, err %v", expiredNow, err)
	}
	if status, _ := api.requestState(application.ID); status != "expired" {
		t.Fatalf("request is %s", status)
	}
	var members int
	api.scalar(&members, `SELECT count(*) FROM project_members WHERE project_id = $1 AND left_at IS NULL`, project.ID)
	if members != 1 || api.notificationCount(lead, notifRequestExpired) != 1 || api.notificationCount(bob, notifRequestExpired) != 1 {
		t.Fatalf("%d members, or a side wasn't told", members)
	}
}

func TestDecisionsRecheckRecruitmentAndTeamState(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	project := api.postProject(lead, "Team", map[string]int{"Dev": 5})
	application := bob.applyTo(project, "Dev")
	path := "/requests/" + application.ID + "/approve"

	api.exec(`UPDATE projects SET recruitment_status = 'closed' WHERE id = $1`, project.ID)
	lead.expectCode(http.StatusConflict, "recruitment_closed", "POST", path, map[string]any{})
	api.exec(`UPDATE projects SET status = 'cancelled', started_at = now(), cancelled_at = now(), expected_end_date = now() WHERE id = $1`, project.ID)
	lead.expectCode(http.StatusConflict, "team_ended", "POST", path, map[string]any{})
	var members int
	api.scalar(&members, `SELECT count(*) FROM project_members WHERE project_id = $1 AND left_at IS NULL`, project.ID)
	if members != 1 {
		t.Fatalf("%d members", members)
	}
}

func TestClosingCancelsAndReopeningDoesNotRestore(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	cara := api.student(caraEmail, "Cara")
	project := api.postProject(lead, "Team", map[string]int{"Dev": 5})
	application := bob.applyTo(project, "Dev")
	invitation := lead.inviteTo(project, "Dev", cara)
	recruitment := "/projects/" + project.ID + "/recruitment"

	var closed struct {
		RecruitmentStatus string `json:"recruitmentStatus"`
	}
	mustDecode(t, lead.expect(http.StatusOK, "POST", recruitment, map[string]any{"status": "closed"}), &closed)
	if closed.RecruitmentStatus != "closed" {
		t.Fatalf("recruitment: %s", closed.RecruitmentStatus)
	}
	for _, id := range []string{application.ID, invitation.ID} {
		if status, reason := api.requestState(id); status != "cancelled" || reason != "recruitment_closed" {
			t.Fatalf("request is %s/%s", status, reason)
		}
	}
	if api.notificationCount(bob, notifRequestCancelled) != 1 || api.notificationCount(cara, notifRequestCancelled) != 1 {
		t.Fatal("students not told")
	}
	lead.expect(http.StatusOK, "POST", recruitment, map[string]any{"status": "open"})
	for _, id := range []string{application.ID, invitation.ID} {
		if status, _ := api.requestState(id); status != "cancelled" {
			t.Fatalf("reopening restored a request: %s", status)
		}
	}
	// Cancelled isn't a refusal, so both can start again.
	bob.applyTo(project, "Dev")
	lead.inviteTo(project, "Dev", cara)
	lead.expect(http.StatusBadRequest, "POST", recruitment, map[string]any{"status": "paused"})
}

func TestRemovingARoleCancelsAndNotifies(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	cara := api.student(caraEmail, "Cara")
	dan := api.student(danEmail, "Dan")
	project := api.postProject(lead, "Team", map[string]int{"Dev": 2, "Ops": 2})
	devPath, opsPath := "/projects/"+project.ID+"/roles/"+project.role(t, "Dev").ID, "/projects/"+project.ID+"/roles/"+project.role(t, "Ops").ID

	lead.decide(dan.applyTo(project, "Dev"), "approve")
	lead.expectCode(http.StatusConflict, "role_has_members", "DELETE", devPath, nil)
	lead.expectCode(http.StatusConflict, "role_has_members", "DELETE", "/projects/"+project.ID+"/roles/"+project.role(t, "Lead").ID, nil)

	application := bob.applyTo(project, "Ops")
	invitation := lead.inviteTo(project, "Ops", cara)
	stays := lead.inviteTo(project, "Dev", api.student("6730000521@student.chula.ac.th", "Fay"))
	lead.expect(http.StatusNoContent, "DELETE", opsPath, nil)
	for _, id := range []string{application.ID, invitation.ID} {
		if status, reason := api.requestState(id); status != "cancelled" || reason != "role_removed" {
			t.Fatalf("request is %s/%s", status, reason)
		}
	}
	if status, _ := api.requestState(stays.ID); status != "pending" {
		t.Fatalf("request on another role is %s", status)
	}
	var message string
	api.scalar(&message, `SELECT data->>'reason' FROM notifications WHERE user_id = $1 AND type = 'request_cancelled'`, bob.userID)
	if message != "role_removed" {
		t.Fatalf("reason %q", message)
	}

	var detail projectJSON
	mustDecode(t, lead.expect(http.StatusOK, "GET", "/projects/"+project.ID, nil), &detail)
	if len(detail.Roles) != 2 || detail.PlacesTotal != 3 || detail.PlacesTaken != 2 {
		t.Fatalf("removed role still counts: %+v", detail)
	}
	bob.expectCode(http.StatusNotFound, "role_not_found", "POST", "/projects/"+project.ID+"/applications", map[string]any{"roleId": project.role(t, "Ops").ID})
	lead.expectCode(http.StatusNotFound, "role_not_found", "DELETE", opsPath, nil)
	// The name is free again once the old role is gone.
	lead.expect(http.StatusOK, "POST", "/projects/"+project.ID+"/roles", map[string]any{"name": "Ops", "capacity": 1})
}

func TestCapacityRules(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	cara := api.student(caraEmail, "Cara")
	dan := api.student(danEmail, "Dan")
	project := api.postProject(lead, "Team", map[string]int{"Dev": 3})
	path := "/projects/" + project.ID + "/roles/" + project.role(t, "Dev").ID

	lead.decide(bob.applyTo(project, "Dev"), "approve")
	pending := cara.applyTo(project, "Dev")
	lead.expect(http.StatusBadRequest, "PATCH", path, map[string]any{"capacity": 0})
	api.moveMember(project.ID, project.role(t, "Dev").ID, dan.userID)
	lead.expectCode(http.StatusConflict, "capacity_below_members", "PATCH", path, map[string]any{"capacity": 1})

	// Lowering to exactly the member count fills the role, with the same effect as an approval.
	var role roleJSON
	mustDecode(t, lead.expect(http.StatusOK, "PATCH", path, map[string]any{"capacity": 2}), &role)
	if role.Capacity != 2 || role.Taken != 2 || role.Available != 0 {
		t.Fatalf("role: %+v", role)
	}
	if status, reason := api.requestState(pending.ID); status != "cancelled" || reason != "role_full" {
		t.Fatalf("pending request is %s/%s", status, reason)
	}
	mustDecode(t, lead.expect(http.StatusOK, "PATCH", path, map[string]any{"name": "Developer", "capacity": 4}), &role)
	if role.Name != "Developer" || role.Available != 2 {
		t.Fatalf("role: %+v", role)
	}
	lead.expectCode(http.StatusConflict, "role_name_taken", "PATCH", path, map[string]any{"name": "lead"})
}

func TestPublishingValidation(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	valid := func() map[string]any {
		return map[string]any{
			"name": "Team", "description": "D", "categoryId": api.categoryID(), "durationValue": 17, "durationUnit": "day",
			"roles": []map[string]any{{"name": "Lead", "capacity": 1}, {"name": "Dev", "capacity": 1}}, "leadRoleIndex": 0,
		}
	}
	for field, value := range map[string]any{
		"name": "  ", "description": "", "categoryId": "00000000-0000-0000-0000-000000000000", "durationValue": 0,
		"durationUnit": "year", "leadRoleIndex": 2,
		"roles": []map[string]any{{"name": "Only me", "capacity": 1}},
	} {
		body := valid()
		body[field] = value
		raw := lead.expect(http.StatusBadRequest, "POST", "/projects", body)
		var parsed struct {
			Error struct {
				Fields map[string]string `json:"fields"`
			} `json:"error"`
		}
		mustDecode(t, raw, &parsed)
		if parsed.Error.Fields[field] == "" {
			t.Errorf("%s: no field error in %s", field, raw)
		}
	}
	unknown := valid()
	unknown["surprise"] = true
	lead.expect(http.StatusBadRequest, "POST", "/projects", unknown)

	var project projectJSON
	mustDecode(t, lead.expect(http.StatusCreated, "POST", "/projects", valid()), &project)
	if len(project.Members) != 1 || project.Members[0].UserID != lead.userID || !project.Members[0].IsLead || !project.Viewer.IsLead || !project.Viewer.IsMember {
		t.Fatalf("creator is not lead and member: %+v", project)
	}
	if project.PlacesTaken != 1 || project.PlacesTotal != 2 || project.role(t, "Lead").Available != 0 || project.role(t, "Dev").Available != 1 {
		t.Fatalf("places: %+v", project)
	}
	if project.Viewer.ApplyBlock == nil || project.Viewer.ApplyBlock.Code != "lead_or_member" {
		t.Fatalf("apply block: %+v", project.Viewer.ApplyBlock)
	}
}

func TestContactLinksOnlyReachCurrentMembers(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	member := api.student(bobEmail, "Member")
	applicant := api.student(caraEmail, "Applicant")
	outsider := api.student(danEmail, "Outsider")
	project := api.postProject(lead, "Team", map[string]int{"Dev": 5})
	const line, discord = "https://line.me/ti/g/secret-line", "https://discord.gg/secret-discord"
	api.exec(`UPDATE projects SET line_url = $2, discord_url = $3 WHERE id = $1`, project.ID, line, discord)
	joined := member.applyTo(project, "Dev")
	lead.decide(joined, "approve")
	applicant.applyTo(project, "Dev")

	leaks := func(raw []byte) bool {
		return strings.Contains(string(raw), "secret-line") || strings.Contains(string(raw), "secret-discord")
	}
	for _, who := range []*testClient{applicant, outsider} {
		raw := who.expect(http.StatusOK, "GET", "/projects/"+project.ID, nil)
		var detail projectJSON
		mustDecode(t, raw, &detail)
		if leaks(raw) || detail.Contact != nil {
			t.Fatalf("%s saw contact links: %s", who.email, raw)
		}
		for _, path := range []string{"/projects?q=Team", "/projects/" + project.ID + "/similar", "/me/requests", "/me/projects"} {
			if raw := who.expect(http.StatusOK, "GET", path, nil); leaks(raw) {
				t.Fatalf("%s leaks contact links: %s", path, raw)
			}
		}
	}
	for _, who := range []*testClient{lead, member} {
		var detail projectJSON
		mustDecode(t, who.expect(http.StatusOK, "GET", "/projects/"+project.ID, nil), &detail)
		if detail.Contact == nil || detail.Contact.LineURL != line {
			t.Fatalf("%s can't see contact links: %+v", who.email, detail.Contact)
		}
	}
	// Lists never carry them, even for members.
	for _, path := range []string{"/projects?q=Team", "/me/projects"} {
		if raw := member.expect(http.StatusOK, "GET", path, nil); leaks(raw) {
			t.Fatalf("%s leaks contact links to a member: %s", path, raw)
		}
	}
	// Leaving takes the links away at once.
	api.exec(`UPDATE project_members SET left_at = now(), left_reason = 'left' WHERE project_id = $1 AND user_id = $2`, project.ID, member.userID)
	if raw := member.expect(http.StatusOK, "GET", "/projects/"+project.ID, nil); leaks(raw) {
		t.Fatalf("former member saw contact links: %s", raw)
	}
}

func TestJoiningDoesNotCancelRequestsToOtherTeams(t *testing.T) {
	api := newTestAPI(t, true)
	first := api.student(aliceEmail, "First")
	second := api.student(caraEmail, "Second")
	third := api.student(danEmail, "Third")
	bob := api.student(bobEmail, "Bob")
	teamA := api.postProject(first, "A", map[string]int{"Dev": 1})
	teamB := api.postProject(second, "B", map[string]int{"Dev": 1})
	teamC := api.postProject(third, "C", map[string]int{"Dev": 1})
	joining := bob.applyTo(teamA, "Dev")
	elsewhere := bob.applyTo(teamB, "Dev")
	invited := third.inviteTo(teamC, "Dev", bob)

	// This fills teamA's only free place, the case that triggers a role_full cascade.
	first.decide(joining, "approve")
	for _, id := range []string{elsewhere.ID, invited.ID} {
		if status, _ := api.requestState(id); status != "pending" {
			t.Fatalf("request to another team is %s", status)
		}
	}
	// Belonging to one team doesn't stop joining another.
	bob.decide(invited, "accept")
	second.decide(elsewhere, "approve")
	var teams struct {
		Items []struct {
			ID       string `json:"id"`
			RoleName string `json:"roleName"`
			IsLead   bool   `json:"isLead"`
		} `json:"items"`
	}
	mustDecode(t, bob.expect(http.StatusOK, "GET", "/me/projects", nil), &teams)
	if len(teams.Items) != 3 || teams.Items[0].RoleName != "Dev" || teams.Items[0].IsLead {
		t.Fatalf("my projects: %+v", teams.Items)
	}
}

func TestProjectSearchReturnsOnlyJoinableTeams(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	joinable := api.postProject(lead, "Joinable Rocket", map[string]int{"Pilot": 1})
	full := api.postProject(lead, "Full Rocket", map[string]int{"Pilot": 1})
	lead.decide(bob.applyTo(full, "Pilot"), "approve")
	closed := api.postProject(lead, "Closed Rocket", map[string]int{"Pilot": 1})
	lead.expect(http.StatusOK, "POST", "/projects/"+closed.ID+"/recruitment", map[string]any{"status": "closed"})
	// Its only free place was in a role that has been removed.
	removed := api.postProject(lead, "Removed Rocket", map[string]int{"Pilot": 1})
	lead.expect(http.StatusNoContent, "DELETE", "/projects/"+removed.ID+"/roles/"+removed.role(t, "Pilot").ID, nil)
	ended := api.postProject(lead, "Ended Rocket", map[string]int{"Pilot": 1})
	api.exec(`UPDATE projects SET status = 'cancelled', recruitment_status = 'closed', started_at = now(), cancelled_at = now(), expected_end_date = now() WHERE id = $1`, ended.ID)
	deleted := api.postProject(lead, "Deleted Rocket", map[string]int{"Pilot": 1})
	api.exec(`UPDATE projects SET recruitment_status = 'closed', deleted_at = now() WHERE id = $1`, deleted.ID)
	other := api.postProject(lead, "Unrelated", map[string]int{"Chef": 1})

	search := func(query string) []string {
		t.Helper()
		var page struct {
			Items []projectJSON `json:"items"`
			Total int           `json:"total"`
		}
		mustDecode(t, bob.expect(http.StatusOK, "GET", "/projects"+query, nil), &page)
		ids := []string{}
		for _, item := range page.Items {
			ids = append(ids, item.ID)
		}
		if page.Total != len(ids) {
			t.Fatalf("%s: total %d with %d items", query, page.Total, len(ids))
		}
		return ids
	}
	check := func(query string, want ...string) {
		t.Helper()
		got := search(query)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("%s: got %v, want %v", query, got, want)
		}
	}
	check("", other.ID, joinable.ID)
	check("?q=rOCKET", joinable.ID)          // name, any case
	check("?q=descr", other.ID, joinable.ID) // description
	check("?q=chE", other.ID)                // role name
	check("?q=%25")                          // % is text, not a wildcard
	check("?q=nothing-matches")              //
	check("?category=not-a-uuid")            //
	var category string
	api.scalar(&category, `SELECT category_id::text FROM projects WHERE id = $1`, joinable.ID)
	check("?category="+category, other.ID, joinable.ID)
	var otherCategory string
	api.scalar(&otherCategory, `SELECT id::text FROM project_categories WHERE id <> $1 LIMIT 1`, category)
	check("?category=" + otherCategory)
	// A removed role's name no longer finds the team, even with another free place.
	lead.expect(http.StatusNoContent, "DELETE", "/projects/"+other.ID+"/roles/"+other.role(t, "Chef").ID, nil)
	check("?q=chE")

	var similar struct {
		Items []projectJSON `json:"items"`
	}
	mustDecode(t, bob.expect(http.StatusOK, "GET", "/projects/"+full.ID+"/similar", nil), &similar)
	if len(similar.Items) != 1 || similar.Items[0].ID != joinable.ID {
		t.Fatalf("similar: %+v", similar.Items)
	}
	bob.expectCode(http.StatusNotFound, "project_not_found", "GET", "/projects/"+deleted.ID, nil)
	bob.expectCode(http.StatusNotFound, "project_not_found", "GET", "/projects/"+deleted.ID+"/similar", nil)
}

func TestEndDateReminderIsWrittenOnceAndOnlyForTheLead(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	project := api.postProject(lead, "Team", map[string]int{"Dev": 2})
	lead.decide(bob.applyTo(project, "Dev"), "approve")
	api.exec(`UPDATE projects SET status = 'in_progress', started_at = now(), expected_end_date = (now() AT TIME ZONE 'Asia/Bangkok')::date WHERE id = $1`, project.ID)

	lead.expect(http.StatusOK, "GET", "/notifications", nil)
	lead.expect(http.StatusOK, "GET", "/notifications", nil)
	if got := api.notificationCount(lead, notifEndDateReached); got != 1 {
		t.Fatalf("%d reminders", got)
	}
	var seen struct {
		EndDateReached bool   `json:"endDateReached"`
		Status         string `json:"status"`
	}
	mustDecode(t, lead.expect(http.StatusOK, "GET", "/projects/"+project.ID, nil), &seen)
	if !seen.EndDateReached || seen.Status != "in_progress" {
		t.Fatalf("lead sees %+v", seen)
	}
	mustDecode(t, bob.expect(http.StatusOK, "GET", "/projects/"+project.ID, nil), &seen)
	if seen.EndDateReached {
		t.Fatal("the banner is for the lead")
	}
}

func TestInvitableProjectsReflectRealTeams(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead")
	bob := api.student(bobEmail, "Bob")
	cara := api.student(caraEmail, "Cara")
	project := api.postProject(lead, "Team", map[string]int{"Dev": 1, "Gone": 1})
	lead.decide(cara.applyTo(project, "Dev"), "approve")
	lead.expect(http.StatusNoContent, "DELETE", "/projects/"+project.ID+"/roles/"+project.role(t, "Gone").ID, nil)
	closed := api.postProject(lead, "Closed", map[string]int{"Dev": 1})
	lead.expect(http.StatusOK, "POST", "/projects/"+closed.ID+"/recruitment", map[string]any{"status": "closed"})

	var profile struct {
		ViewerActions struct {
			InvitableProjects []struct {
				ID    string     `json:"id"`
				Roles []roleJSON `json:"roles"`
			} `json:"invitableProjects"`
		} `json:"viewerActions"`
	}
	mustDecode(t, lead.expect(http.StatusOK, "GET", "/profiles/"+bob.userID, nil), &profile)
	projects := profile.ViewerActions.InvitableProjects
	if len(projects) != 1 || projects[0].ID != project.ID || len(projects[0].Roles) != 2 {
		t.Fatalf("invitable: %+v", projects)
	}
	for _, role := range projects[0].Roles {
		if role.Available != 0 {
			t.Fatalf("every remaining role is full: %+v", role)
		}
	}
	mustDecode(t, lead.expect(http.StatusOK, "GET", "/profiles/"+cara.userID, nil), &profile)
	if len(profile.ViewerActions.InvitableProjects) != 0 {
		t.Fatalf("a current member's team is invitable: %+v", profile.ViewerActions.InvitableProjects)
	}
	mustDecode(t, bob.expect(http.StatusOK, "GET", "/profiles/"+lead.userID, nil), &profile)
	if len(profile.ViewerActions.InvitableProjects) != 0 {
		t.Fatalf("a non-lead has invitable teams: %+v", profile.ViewerActions.InvitableProjects)
	}
}

func TestNotificationsRenderFromRequestRows(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Lead Person")
	bob := api.student(bobEmail, "Bob Person")
	project := api.postProject(lead, "Rocket Team", map[string]int{"Dev": 2})
	application := bob.applyTo(project, "Dev")

	var inbox struct {
		Items []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Link    string `json:"link"`
		} `json:"items"`
	}
	mustDecode(t, lead.expect(http.StatusOK, "GET", "/notifications", nil), &inbox)
	if len(inbox.Items) != 1 || inbox.Items[0].Message != "Bob Person applied for Dev on Rocket Team." || inbox.Items[0].Link != "/requests/"+application.ID {
		t.Fatalf("inbox: %+v", inbox.Items)
	}
	lead.expect(http.StatusOK, "POST", "/projects/"+project.ID+"/recruitment", map[string]any{"status": "closed"})
	mustDecode(t, bob.expect(http.StatusOK, "GET", "/notifications", nil), &inbox)
	if len(inbox.Items) != 1 || inbox.Items[0].Message != "Rocket Team closed recruitment, so your request was cancelled." {
		t.Fatalf("inbox: %+v", inbox.Items)
	}
}

func TestSeededDemoWorksThroughTheAPI(t *testing.T) {
	api := newTestAPI(t, true)
	if err := seed.Run(context.Background(), api.pool); err != nil {
		t.Fatal(err)
	}
	mint := api.signIn("6730123421@student.chula.ac.th")
	ploy := api.signIn("6830456726@student.chula.ac.th")
	tonkla := api.signIn("6630789035@student.chula.ac.th")
	beam := api.signIn("6730567823@student.chula.ac.th")
	outsider := api.signIn("6730234528@student.chula.ac.th")
	var projectID string
	api.scalar(&projectID, `SELECT id FROM projects WHERE name='Campus Study Planner'`)
	path := "/projects/" + projectID + "/ratings"

	var published ProjectRatings
	mustDecode(t, outsider.expect(http.StatusOK, "GET", path, nil), &published)
	if published.Open || published.Eligible || len(published.Teammates) != 0 || published.StatusText != nil || len(published.Published) != 3 {
		t.Fatalf("published ratings or outsider eligibility: %+v", published)
	}
	want := map[string]int{mint.userID + ":" + ploy.userID: 5, mint.userID + ":" + tonkla.userID: 4, ploy.userID + ":" + mint.userID: 4}
	for _, rating := range published.Published {
		key := rating.Reviewer.ID + ":" + rating.Recipient.ID
		if score, ok := want[key]; !ok || rating.Score != score {
			t.Fatalf("unexpected published rating: %+v", rating)
		}
		delete(want, key)
	}
	if len(want) != 0 {
		t.Fatalf("missing published ratings: %+v", want)
	}
	// Receiving a score never creates a submission for that student's teammates.
	var receivedOnly ProjectRatings
	mustDecode(t, tonkla.expect(http.StatusOK, "GET", path, nil), &receivedOnly)
	if !receivedOnly.Eligible || receivedOnly.Open || len(receivedOnly.Teammates) != 3 {
		t.Fatalf("completed member eligibility: %+v", receivedOnly)
	}
	for _, teammate := range receivedOnly.Teammates {
		if teammate.MyScore != nil {
			t.Fatalf("missing submission became a score: %+v", teammate)
		}
	}
	for _, example := range []struct {
		student *testClient
		score   float64
		count   int
	}{{mint, 4, 1}, {ploy, 5, 1}, {tonkla, 4, 1}, {beam, 0, 0}} {
		var profile struct {
			Rating    Rating
			Published []any
		}
		mustDecode(t, outsider.expect(http.StatusOK, "GET", "/profiles/"+example.student.userID+"/ratings", nil), &profile)
		if profile.Rating.Count != example.count || len(profile.Published) != example.count {
			t.Fatalf("wrong published count for %s: %+v", example.student.userID, profile)
		}
		if example.count == 0 {
			if profile.Rating.Average != nil {
				t.Fatalf("missing rating counted as zero: %+v", profile.Rating)
			}
		} else if profile.Rating.Average == nil || *profile.Rating.Average != example.score {
			t.Fatalf("missing submissions reduced the average: %+v", profile.Rating)
		}
	}
	expectCode(t, mint, http.StatusConflict, "rating_window_closed", "PUT", path+"/"+beam.userID, map[string]int{"score": 3})
	expectCode(t, outsider, http.StatusForbidden, "not_allowed", "PUT", path+"/"+beam.userID, map[string]int{"score": 3})
	// Seeded CVs obey the same privacy checks as an uploaded CV.
	expectCode(t, outsider, http.StatusForbidden, "cv_not_shared", "GET", "/users/"+ploy.userID+"/cv", nil)
	ploy.expect(http.StatusOK, "GET", "/users/"+ploy.userID+"/cv", nil)
}
