package httpapi

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func decodeDetail(t *testing.T, raw []byte) ProjectDetail {
	t.Helper()
	var d ProjectDetail
	mustDecode(t, raw, &d)
	return d
}
func (a *testAPI) runningTeam(lead *testClient, name string) (string, string) {
	a.t.Helper()
	id, role := a.insertTeam(lead, name, time.Time{})
	a.exec(`UPDATE projects SET status='in_progress',started_at=now()-interval '2 days',expected_end_date=(now() AT TIME ZONE 'Asia/Bangkok')::date+1,recruitment_status='closed' WHERE id=$1`, id)
	return id, role
}
func expectCode(t *testing.T, c *testClient, status int, code, method, path string, body any) {
	t.Helper()
	raw := c.expect(status, method, path, body)
	if got := errorCode(t, raw); got != code {
		t.Fatalf("%s %s: code %s want %s", method, path, got, code)
	}
}

func TestContactPrivacyAndSupportedLinks(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Alice")
	member := api.student(bobEmail, "Bob")
	outsider := api.student(caraEmail, "Cara")
	id, role := api.insertTeam(lead, "Contact", time.Time{})
	api.addMember(id, role, member)
	for _, in := range []map[string]any{
		{"lineUrl": "http://line.me/ti/g/x"}, {"lineUrl": "https://line.me.evil.test/ti/g/x"}, {"lineUrl": "https://line.me@evil.test/ti/g/x"}, {"lineUrl": "https://someone@line.me/ti/g/x"}, {"lineUrl": "https://line.me:8443/ti/g/x"}, {"discordUrl": "https://discord.com.evil.test/invite/x"}, {"discordUrl": "https://discord.com/invite"}, {"discordUrl": "https://discord.gg"}, {"discordUrl": "https://discord.com/profile/x"}, {"lineUrl": "https://line.me./ti/g/x"},
	} {
		expectCode(t, lead, 400, "unsupported_link", "PUT", "/projects/"+id+"/contact", in)
	}
	contact := map[string]any{"lineUrl": "https://line.me/ti/g/test", "discordUrl": "https://discord.com:443/invite/test"}
	expectCode(t, member, 403, "not_lead", "PUT", "/projects/"+id+"/contact", contact)
	lead.expect(200, "PUT", "/projects/"+id+"/contact", contact)
	if d := decodeDetail(t, outsider.expect(200, "GET", "/projects/"+id, nil)); d.Contact != nil {
		t.Fatal("outsider received private link")
	}
	if d := decodeDetail(t, member.expect(200, "GET", "/projects/"+id, nil)); d.Contact == nil || d.Contact.LineURL == nil {
		t.Fatal("member cannot see link")
	}
	lead.expect(204, "DELETE", "/projects/"+id+"/members/"+member.userID, nil)
	if d := decodeDetail(t, member.expect(200, "GET", "/projects/"+id, nil)); d.Contact != nil || d.Viewer.IsMember {
		t.Fatal("removed member retained private access")
	}
}

func TestLifecycleTransitionsFreezeMembershipAndCancelPending(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Alice")
	member := api.student(bobEmail, "Bob")
	outsider := api.student(caraEmail, "Cara")
	id, role := api.insertTeam(lead, "Lifecycle", time.Time{})
	api.addMember(id, role, member)
	tomorrow := time.Now().In(time.FixedZone("Bangkok", 7*3600)).AddDate(0, 0, 1).Format("2006-01-02")
	expectCode(t, lead, 409, "recruitment_open", "POST", "/projects/"+id+"/start", map[string]string{"expectedEndDate": tomorrow})
	lead.expect(200, "POST", "/projects/"+id+"/recruitment", map[string]string{"status": "closed"})
	expectCode(t, member, 403, "not_lead", "POST", "/projects/"+id+"/start", map[string]string{"expectedEndDate": tomorrow})
	d := decodeDetail(t, lead.expect(200, "POST", "/projects/"+id+"/start", map[string]string{"expectedEndDate": tomorrow}))
	if d.Status != "in_progress" || d.PlacesTaken >= d.PlacesTotal {
		t.Fatal("start should allow unfilled slots")
	}
	api.exec(`UPDATE projects SET end_reminder_sent_at=now() WHERE id=$1`, id)
	later := time.Now().In(time.FixedZone("Bangkok", 7*3600)).AddDate(0, 0, 3).Format("2006-01-02")
	lead.expect(200, "POST", "/projects/"+id+"/extend", map[string]string{"expectedEndDate": later})
	var reminder *time.Time
	api.scalar(&reminder, `SELECT end_reminder_sent_at FROM projects WHERE id=$1`, id)
	if reminder != nil {
		t.Fatal("extend retained reminder")
	}
	lead.expect(200, "POST", "/projects/"+id+"/recruitment", map[string]string{"status": "open"})
	outsider.expect(201, "POST", "/projects/"+id+"/applications", map[string]string{"roleId": role})
	transfer := lead.expect(201, "POST", "/projects/"+id+"/transfers", map[string]string{"toUserId": member.userID})
	var tr Transfer
	mustDecode(t, transfer, &tr)
	d = decodeDetail(t, lead.expect(200, "POST", "/projects/"+id+"/complete", map[string]any{}))
	if d.Status != "completed" || d.CompletedAt == nil || d.RecruitmentStatus != "closed" || d.PendingTransfer != nil || d.RatingWindow == nil || !d.RatingWindow.Open {
		t.Fatalf("bad completion: %+v", d)
	}
	var reason string
	api.scalar(&reason, `SELECT cancel_reason::text FROM join_requests WHERE project_id=$1`, id)
	if reason != "project_completed" {
		t.Fatal(reason)
	}
	var prompts int
	api.scalar(&prompts, `SELECT count(*) FROM notifications WHERE project_id=$1 AND type='rate_teammates'`, id)
	if prompts != 2 {
		t.Fatalf("%d rating prompts", prompts)
	}
	for _, action := range []struct {
		c            *testClient
		method, path string
	}{{member, "POST", "/leave"}, {lead, "DELETE", "/members/" + member.userID}, {lead, "POST", "/recruitment"}, {lead, "POST", "/extend"}} {
		body := map[string]any{}
		if action.path == "/recruitment" {
			body["status"] = "open"
		}
		if action.path == "/extend" {
			body["expectedEndDate"] = later
		}
		expectCode(t, action.c, 409, "team_ended", action.method, "/projects/"+id+action.path, body)
	}
	expectCode(t, member, 409, "team_ended", "POST", "/transfers/"+tr.ID+"/accept", map[string]any{})
	expectCode(t, lead, 409, "project_started", "DELETE", "/projects/"+id, nil)
	lead.expect(200, "PUT", "/projects/"+id+"/contact", map[string]any{"lineUrl": "https://line.me/ti/g/done"})
	cancelled, cancelledRole := api.runningTeam(lead, "Cancelled")
	api.addMember(cancelled, cancelledRole, member)
	decodeDetail(t, lead.expect(200, "POST", "/projects/"+cancelled+"/cancel", map[string]any{}))
	expectCode(t, lead, 409, "project_not_completed", "GET", "/projects/"+cancelled+"/ratings", nil)
	expectCode(t, member, 409, "team_ended", "POST", "/projects/"+cancelled+"/leave", map[string]any{})
	expectCode(t, lead, 409, "team_ended", "DELETE", "/projects/"+cancelled+"/members/"+member.userID, nil)
}

func TestTransfersRequireAcceptedCurrentMemberAndKeepOneLead(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Alice")
	member := api.student(bobEmail, "Bob")
	outsider := api.student(caraEmail, "Cara")
	id, role := api.insertTeam(lead, "Transfer", time.Time{})
	api.addMember(id, role, member)
	expectCode(t, lead, 403, "not_allowed", "POST", "/projects/"+id+"/transfers", map[string]string{"toUserId": strings.ToUpper(lead.userID)})
	expectCode(t, lead, 403, "not_allowed", "DELETE", "/projects/"+id+"/members/"+strings.ToUpper(lead.userID), nil)
	expectCode(t, lead, 404, "member_not_found", "POST", "/projects/"+id+"/transfers", map[string]string{"toUserId": outsider.userID})
	var tr Transfer
	mustDecode(t, lead.expect(201, "POST", "/projects/"+id+"/transfers", map[string]string{"toUserId": member.userID}), &tr)
	expectCode(t, lead, 409, "lead_must_transfer", "POST", "/projects/"+id+"/leave", map[string]any{})
	expectCode(t, lead, 409, "transfer_pending", "POST", "/projects/"+id+"/transfers", map[string]string{"toUserId": member.userID})
	expectCode(t, outsider, 404, "transfer_not_found", "POST", "/transfers/"+tr.ID+"/accept", map[string]any{})
	expectCode(t, lead, 403, "not_allowed", "POST", "/transfers/"+tr.ID+"/accept", map[string]any{})
	member.expect(200, "POST", "/transfers/"+tr.ID+"/accept", map[string]any{})
	d := decodeDetail(t, member.expect(200, "GET", "/projects/"+id, nil))
	count := 0
	for _, m := range d.Members {
		if m.IsLead {
			count++
		}
	}
	if d.Lead.ID != member.userID || count != 1 {
		t.Fatal("transfer did not retain exactly one lead")
	}
	// Decline and cancellation never change the lead, and only the correct side can act.
	mustDecode(t, member.expect(201, "POST", "/projects/"+id+"/transfers", map[string]string{"toUserId": lead.userID}), &tr)
	expectCode(t, member, 403, "not_allowed", "POST", "/transfers/"+tr.ID+"/decline", map[string]any{})
	lead.expect(200, "POST", "/transfers/"+tr.ID+"/decline", map[string]any{})
	mustDecode(t, member.expect(201, "POST", "/projects/"+id+"/transfers", map[string]string{"toUserId": lead.userID}), &tr)
	expectCode(t, lead, 403, "not_allowed", "POST", "/transfers/"+tr.ID+"/cancel", map[string]any{})
	member.expect(200, "POST", "/transfers/"+tr.ID+"/cancel", map[string]any{})
	lead.expect(204, "POST", "/projects/"+id+"/leave", map[string]any{})
	// A recipient leaving cancels its transfer and does not reopen recruitment.
	api.addMember(id, role, outsider)
	member.expect(200, "POST", "/projects/"+id+"/recruitment", map[string]string{"status": "closed"})
	mustDecode(t, member.expect(201, "POST", "/projects/"+id+"/transfers", map[string]string{"toUserId": outsider.userID}), &tr)
	outsider.expect(204, "POST", "/projects/"+id+"/leave", map[string]any{})
	d = decodeDetail(t, member.expect(200, "GET", "/projects/"+id, nil))
	if d.PendingTransfer != nil || d.RecruitmentStatus != "closed" {
		t.Fatal("leave reopened recruitment or retained transfer")
	}
	expectCode(t, outsider, 409, "transfer_not_pending", "POST", "/transfers/"+tr.ID+"/accept", map[string]any{})
}

func TestDeleteOnlyUnstartedAndRevokesAllMemberships(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Alice")
	member := api.student(bobEmail, "Bob")
	applicant := api.student(caraEmail, "Cara")
	id, role := api.insertTeam(lead, "Delete", time.Time{})
	api.addMember(id, role, member)
	applicant.expect(201, "POST", "/projects/"+id+"/applications", map[string]string{"roleId": role})
	lead.expect(201, "POST", "/projects/"+id+"/transfers", map[string]string{"toUserId": member.userID})
	lead.expect(204, "DELETE", "/projects/"+id, nil)
	expectCode(t, member, 404, "project_not_found", "GET", "/projects/"+id, nil)
	var current int
	api.scalar(&current, `SELECT count(*) FROM project_members WHERE project_id=$1 AND left_at IS NULL`, id)
	if current != 0 {
		t.Fatal("delete retained memberships")
	}
	raw := member.expect(200, "GET", "/notifications", nil)
	if strings.Contains(string(raw), `"link":"/projects/`+id) {
		t.Fatal("deleted project notification links to missing project")
	}
}

func TestRatingEligibilityUpsertPublicationAndMissingScores(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Alice")
	member := api.student(bobEmail, "Bob")
	former := api.student(caraEmail, "Cara")
	outsider := api.student(danEmail, "Dan")
	id, role := api.runningTeam(lead, "Ratings")
	api.addMember(id, role, member)
	api.addMember(id, role, former)
	former.expect(204, "POST", "/projects/"+id+"/leave", map[string]any{})
	lead.expect(200, "POST", "/projects/"+id+"/complete", map[string]any{})
	path := "/projects/" + id + "/ratings/"
	for _, c := range []*testClient{former, outsider} {
		expectCode(t, c, 403, "not_allowed", "PUT", path+member.userID, map[string]int{"score": 3})
	}
	for _, target := range []string{lead.userID, former.userID, outsider.userID} {
		expectCode(t, lead, 403, "not_allowed", "PUT", path+target, map[string]int{"score": 3})
	}
	for _, body := range []any{map[string]any{}, map[string]int{"score": 0}, map[string]int{"score": 6}} {
		expectCode(t, lead, 400, "score_required", "PUT", path+member.userID, body)
	}
	lead.expect(200, "PUT", path+member.userID, map[string]int{"score": 3})
	lead.expect(200, "PUT", path+member.userID, map[string]int{"score": 5})
	var rows int
	api.scalar(&rows, `SELECT count(*) FROM ratings WHERE project_id=$1`, id)
	if rows != 1 {
		t.Fatal("upsert duplicated rating")
	}
	var result ProjectRatings
	mustDecode(t, lead.expect(200, "GET", "/projects/"+id+"/ratings", nil), &result)
	if !result.Eligible || !result.Open || len(result.Teammates) != 1 || result.Teammates[0].MyScore == nil || *result.Teammates[0].MyScore != 5 || len(result.Published) != 0 {
		t.Fatalf("reviewer read: %+v", result)
	}
	mustDecode(t, outsider.expect(200, "GET", "/projects/"+id+"/ratings", nil), &result)
	if result.Eligible || len(result.Teammates) != 0 || len(result.Published) != 0 || result.StatusText == nil {
		t.Fatal("outsider saw pending scores")
	}
	var profile struct {
		Rating          Rating
		Published       []any
		PendingProjects []any
	}
	mustDecode(t, outsider.expect(200, "GET", "/profiles/"+member.userID+"/ratings", nil), &profile)
	if profile.Rating.Count != 0 || profile.Rating.Average != nil || len(profile.Published) != 0 {
		t.Fatal("pending scores entered profile average")
	}
	// Exactly 14 days has elapsed at transaction time. The boundary is closed.
	api.exec(`UPDATE projects SET completed_at=now()-interval '14 days',started_at=started_at-interval '20 days' WHERE id=$1`, id)
	api.exec(`UPDATE project_members SET joined_at=joined_at-interval '20 days',left_at=left_at-interval '20 days' WHERE project_id=$1`, id)
	expectCode(t, lead, 409, "rating_window_closed", "PUT", path+member.userID, map[string]int{"score": 2})
	mustDecode(t, outsider.expect(200, "GET", "/projects/"+id+"/ratings", nil), &result)
	if result.Open || len(result.Published) != 1 || result.Published[0].Score != 5 {
		t.Fatal("day14 ratings not published")
	}
	mustDecode(t, outsider.expect(200, "GET", "/profiles/"+member.userID+"/ratings", nil), &profile)
	if profile.Rating.Count != 1 || profile.Rating.Average == nil || *profile.Rating.Average != 5 {
		t.Fatal("published average is wrong")
	}
	raw := outsider.expect(200, "GET", "/profiles/"+lead.userID+"/teammates", nil)
	if !strings.Contains(string(raw), `"state":"not_rated"`) {
		t.Fatal("missing rating was not displayed")
	}
	member.expect(204, "POST", "/me/close", map[string]any{})
	mustDecode(t, outsider.expect(200, "GET", "/projects/"+id+"/ratings", nil), &result)
	if len(result.Published) != 0 {
		t.Fatal("closed account rating remained visible")
	}
	var frozen int
	api.scalar(&frozen, `SELECT count(*) FROM project_members WHERE project_id=$1 AND user_id=$2 AND left_at IS NULL`, id, member.userID)
	if frozen != 1 {
		t.Fatal("account closure changed terminal membership")
	}
}

func TestCloseRacesCannotCreateClosedLeadOrMembership(t *testing.T) {
	for _, scenario := range []string{"transfer", "project", "complete"} {
		t.Run(scenario, func(t *testing.T) {
			api := newTestAPI(t, true)
			lead := api.student(aliceEmail, "Alice")
			member := api.student(bobEmail, "Bob")
			id, role := api.runningTeam(lead, "Race")
			api.addMember(id, role, member)
			var method, path string
			var body any
			switch scenario {
			case "transfer":
				var tr Transfer
				mustDecode(t, lead.expect(201, "POST", "/projects/"+id+"/transfers", map[string]string{"toUserId": member.userID}), &tr)
				method, path, body = "POST", "/transfers/"+tr.ID+"/accept", map[string]any{}
			case "project":
				method, path, body = "POST", "/projects", map[string]any{"name": "Concurrent project", "description": "Test", "categoryId": api.categoryID(), "durationValue": 1, "durationUnit": "day", "roles": []map[string]any{{"name": "Builder", "capacity": 2}}, "leadRoleIndex": 0}
			case "complete":
				method, path, body = "POST", "/projects/"+id+"/complete", map[string]any{}
			}
			var wg sync.WaitGroup
			start := make(chan struct{})
			var statuses [2]int
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				actor := member
				if scenario == "complete" {
					actor = lead
				}
				response, _ := actor.send(method, path, body)
				statuses[0] = response.StatusCode
			}()
			go func() {
				defer wg.Done()
				<-start
				response, _ := member.send("POST", "/me/close", map[string]any{})
				statuses[1] = response.StatusCode
			}()
			close(start)
			wg.Wait()
			for _, status := range statuses {
				if status >= 500 {
					t.Fatalf("race failed: %v", statuses)
				}
			}
			var invalid int
			api.scalar(&invalid, `SELECT count(*) FROM projects pr JOIN users u ON u.id=pr.lead_user_id WHERE u.closed_at IS NOT NULL AND pr.deleted_at IS NULL AND pr.status IN ('not_started','in_progress')`)
			if invalid != 0 {
				t.Fatal("closed account became live lead")
			}
			api.scalar(&invalid, `SELECT count(*) FROM project_members m JOIN projects pr ON pr.id=m.project_id JOIN users u ON u.id=m.user_id WHERE u.closed_at IS NOT NULL AND m.left_at IS NULL AND pr.status IN ('not_started','in_progress')`)
			if invalid != 0 {
				t.Fatal("closed account retained active membership")
			}
			if scenario == "complete" {
				var completedAt time.Time
				api.scalar(&completedAt, `SELECT completed_at FROM projects WHERE id=$1`, id)
				var leftAt *time.Time
				api.scalar(&leftAt, `SELECT left_at FROM project_members WHERE project_id=$1 AND user_id=$2`, id, member.userID)
				if leftAt != nil && leftAt.After(completedAt) {
					t.Fatal("closure changed membership after completion")
				}
			}
			// Clean transaction state is also evidence that the competing operations finished without a lock leak.
			if err := api.pool.Ping(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRatingEditWaitingForAnotherEditCannotOutlastWindow(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.student(aliceEmail, "Alice")
	member := api.student(bobEmail, "Bob")
	id, role := api.runningTeam(lead, "Rating deadline race")
	api.addMember(id, role, member)
	lead.expect(200, "POST", "/projects/"+id+"/complete", map[string]any{})
	path := "/projects/" + id + "/ratings/" + member.userID
	lead.expect(200, "PUT", path, map[string]int{"score": 1})
	ctx := context.Background()
	blocker, err := api.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(ctx)
	if _, err := blocker.Exec(ctx, `UPDATE ratings SET score=1 WHERE project_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	var closesAt time.Time
	api.scalar(&closesAt, `UPDATE projects SET completed_at=clock_timestamp()-interval '14 days'+interval '1 second' WHERE id=$1 RETURNING completed_at+interval '14 days'`, id)
	type outcome struct {
		status int
		body   []byte
	}
	result := make(chan outcome, 1)
	go func() {
		response, raw := lead.send("PUT", path, map[string]int{"score": 5})
		result <- outcome{response.StatusCode, raw}
	}()
	// Make sure the request reached the competing rating write before the deadline.
	for {
		var blocked bool
		api.scalar(&blocked, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'INSERT INTO ratings(project_id,reviewer_id,recipient_id,score)%')`)
		if blocked {
			break
		}
		if time.Now().After(closesAt) {
			t.Fatal("rating write did not reach lock before deadline")
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(time.Until(closesAt) + 10*time.Millisecond)
	if err := blocker.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	got := <-result
	if got.status != 409 || errorCode(t, got.body) != "rating_window_closed" {
		t.Fatalf("waiting edit returned %d: %s", got.status, got.body)
	}
	var score int
	api.scalar(&score, `SELECT score FROM ratings WHERE project_id=$1`, id)
	if score != 1 {
		t.Fatal("waiting edit changed score after deadline")
	}
}
