package seed

import (
	"context"
	"testing"

	"github.com/thinc-org/CU-Profile/apps/api/internal/demo"
	"github.com/thinc-org/CU-Profile/apps/api/internal/testdb"
)

func TestResetOnlyAllowedOnThisMachine(t *testing.T) {
	for url, allowed := range map[string]bool{
		"postgresql://u:p@localhost:5432/db":                true,
		"postgresql://u:p@127.0.0.1:5432/db":                true,
		"postgresql://u:p@[::1]:5432/db":                    true,
		"postgresql://u:p@db.example.com:5432/db":           false,
		"postgresql://u:p@localhost.example.com/db":         false,
		"postgresql://u:p@localhost,db.example.com/db":      false,
		"postgresql://u:p@localhost/db?host=db.example.com": false,
		"postgresql://u:p@/db?host=/var/run/postgresql":     false,
	} {
		if err := CheckResetAllowed(url); (err == nil) != allowed {
			t.Errorf("%s: allowed=%v, err=%v", url, allowed, err)
		}
		if err := CheckAutomaticSeedAllowed(url); (err == nil) != allowed {
			t.Errorf("automatic seed %s: allowed=%v, err=%v", url, allowed, err)
		}
	}
}

func TestAutomaticSeedPreservesExistingData(t *testing.T) {
	t.Run("concurrent first start and repeated startup", func(t *testing.T) {
		pool := testdb.New(t)
		ctx := context.Background()
		type result struct {
			seeded bool
			err    error
		}
		results := make(chan result, 2)
		for range 2 {
			go func() {
				seeded, err := RunIfEmpty(ctx, pool)
				results <- result{seeded, err}
			}()
		}
		seededCount := 0
		for range 2 {
			out := <-results
			if out.err != nil {
				t.Fatal(out.err)
			}
			if out.seeded {
				seededCount++
			}
		}
		if seededCount != 1 {
			t.Fatalf("seed ran %d times during concurrent startup", seededCount)
		}
		if _, err := pool.Exec(ctx, `UPDATE profiles SET display_name='Edited demo profile',cv_shared=false
			WHERE user_id=(SELECT id FROM users WHERE email=$1)`, mint); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE users SET closed_at=now() WHERE email=$1`, mint); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE ratings SET score=2`); err != nil {
			t.Fatal(err)
		}
		snapshot := func() string {
			t.Helper()
			var value string
			if err := pool.QueryRow(ctx, `SELECT json_build_array(
				(SELECT json_agg(u ORDER BY id) FROM users u),
				(SELECT json_agg(p ORDER BY user_id) FROM profiles p),
				(SELECT json_agg(r ORDER BY id) FROM ratings r))::text`).Scan(&value); err != nil {
				t.Fatal(err)
			}
			return value
		}
		before := snapshot()
		if seeded, err := RunIfEmpty(ctx, pool); err != nil || seeded {
			t.Fatalf("repeated startup seeded=%v, err=%v", seeded, err)
		}
		if snapshot() != before {
			t.Fatal("startup overwrote a profile, reopened an account or changed ratings")
		}
	})
	t.Run("existing non-demo account", func(t *testing.T) {
		pool := testdb.New(t)
		ctx := context.Background()
		if _, err := pool.Exec(ctx, `INSERT INTO users(email,oauth_subject)
			VALUES('6730009921@student.chula.ac.th','existing-student')`); err != nil {
			t.Fatal(err)
		}
		if seeded, err := RunIfEmpty(ctx, pool); err != nil || seeded {
			t.Fatalf("existing database seeded=%v, err=%v", seeded, err)
		}
		var users, projects int
		if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM users),(SELECT count(*) FROM projects)`).Scan(&users, &projects); err != nil {
			t.Fatal(err)
		}
		if users != 1 || projects != 0 {
			t.Fatal("startup added demo data to an existing database")
		}
	})
}

// The seed writes directly to the database, so it must preserve the API's invariants.
func TestSeededTeamsKeepTheAppsInvariants(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	if err := Run(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for name, sql := range map[string]string{
		"lead is not a current member": `SELECT count(*) FROM projects pr WHERE NOT EXISTS (
			SELECT 1 FROM project_members m WHERE m.project_id = pr.id AND m.user_id = pr.lead_user_id AND m.left_at IS NULL)`,
		"role over capacity": `SELECT count(*) FROM project_roles r
			WHERE r.capacity < (SELECT count(*) FROM project_members m WHERE m.role_id = r.id AND m.left_at IS NULL)`,
		"invalid project lifecycle": `SELECT count(*) FROM projects WHERE status <> 'completed' OR recruitment_status <> 'closed'
			OR completed_at IS NULL OR started_at IS NULL OR NOT (created_at < started_at AND started_at < completed_at)
			OR completed_at + interval '14 days' > now()
			OR expected_end_date <> (completed_at AT TIME ZONE 'Asia/Bangkok')::date`,
		"invalid membership timeline": `SELECT count(*) FROM project_members m JOIN projects p ON p.id=m.project_id
			WHERE m.left_at IS NOT NULL OR m.joined_at < p.created_at OR m.joined_at >= p.completed_at`,
		"invalid rating timeline": `SELECT count(*) FROM ratings r JOIN projects p ON p.id=r.project_id
			WHERE r.created_at < p.completed_at OR r.created_at >= p.completed_at + interval '14 days'
			OR r.updated_at < r.created_at OR r.updated_at >= p.completed_at + interval '14 days'`,
		"ineligible rating pair": `SELECT count(*) FROM ratings r WHERE r.reviewer_id=r.recipient_id OR
			(SELECT count(*) FROM project_members m WHERE m.project_id=r.project_id AND m.left_at IS NULL
			AND m.user_id IN (r.reviewer_id,r.recipient_id)) <> 2`,
	} {
		var broken int
		if err := pool.QueryRow(ctx, sql).Scan(&broken); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if broken != 0 {
			t.Errorf("%s: %d rows", name, broken)
		}
	}
	var projects, roles, members, ratings, requests, notifications, transfers int
	err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM projects), (SELECT count(*) FROM project_roles),
		(SELECT count(*) FROM project_members), (SELECT count(*) FROM ratings),
		(SELECT count(*) FROM join_requests), (SELECT count(*) FROM notifications),
		(SELECT count(*) FROM leadership_transfers)`).Scan(&projects, &roles, &members, &ratings, &requests, &notifications, &transfers)
	if err != nil {
		t.Fatal(err)
	}
	if projects != 1 || roles != 3 || members != 4 || ratings != 3 || requests != 0 || notifications != 0 || transfers != 0 {
		t.Fatalf("demo rows: %d projects, %d roles, %d members, %d ratings, %d requests, %d notifications, %d transfers",
			projects, roles, members, ratings, requests, notifications, transfers)
	}
}

func TestSeedIsIdempotentAndResetKeepsStarterLists(t *testing.T) {
	pool := testdb.New(t)
	ctx := context.Background()
	counts := func() (users, skills, userSkills, slots, profiles, cvs int) {
		t.Helper()
		err := pool.QueryRow(ctx, `SELECT
			(SELECT count(*) FROM users), (SELECT count(*) FROM skills), (SELECT count(*) FROM user_skills),
			(SELECT count(*) FROM availability_slots), (SELECT count(*) FROM profiles),
			(SELECT count(*) FROM cv_files)`).Scan(&users, &skills, &userSkills, &slots, &profiles, &cvs)
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	starterLists := func() (faculties, categories, predefined int) {
		t.Helper()
		if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM faculties),
			(SELECT count(*) FROM project_categories), (SELECT count(*) FROM skills WHERE is_predefined)`).Scan(&faculties, &categories, &predefined); err != nil {
			t.Fatal(err)
		}
		return
	}
	f1, c1, p1 := starterLists()
	if f1 == 0 || c1 == 0 || p1 == 0 {
		t.Fatal("starter lists are empty before seeding")
	}
	if err := Run(ctx, pool); err != nil {
		t.Fatal(err)
	}
	u1, s1, us1, a1, profiles1, cvs1 := counts()
	if u1 != len(demo.Accounts) || profiles1 != 8 || cvs1 != 4 || us1 == 0 || a1 == 0 {
		t.Fatalf("incomplete demo accounts: %d users, %d profiles, %d CVs, %d skills, %d slots", u1, profiles1, cvs1, us1, a1)
	}
	var seeded int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE email = ANY($1)`, demo.Emails()).Scan(&seeded); err != nil || seeded != len(demo.Accounts) {
		t.Fatalf("demo list and seed disagree: %d seeded of %d, err %v", seeded, len(demo.Accounts), err)
	}
	// Preserve an edited score as well as all original project and membership timestamps.
	if _, err := pool.Exec(ctx, `UPDATE ratings SET score=3,updated_at=updated_at+interval '1 hour'
		WHERE reviewer_id=(SELECT id FROM users WHERE email=$1) AND recipient_id=(SELECT id FROM users WHERE email=$2)`, mint, ploy); err != nil {
		t.Fatal(err)
	}
	projectSnapshot := func() string {
		t.Helper()
		var snapshot string
		err := pool.QueryRow(ctx, `SELECT json_build_array(
			(SELECT json_agg(p ORDER BY id) FROM projects p),
			(SELECT json_agg(r ORDER BY id) FROM project_roles r),
			(SELECT json_agg(m ORDER BY id) FROM project_members m),
			(SELECT json_agg(r ORDER BY id) FROM ratings r))::text`).Scan(&snapshot)
		if err != nil {
			t.Fatal(err)
		}
		return snapshot
	}
	before := projectSnapshot()
	if err := Run(ctx, pool); err != nil {
		t.Fatal(err)
	}
	u2, s2, us2, a2, profiles2, cvs2 := counts()
	if u1 != u2 || s1 != s2 || us1 != us2 || a1 != a2 || profiles1 != profiles2 || cvs1 != cvs2 {
		t.Fatal("second run changed account data counts")
	}
	if after := projectSnapshot(); before != after {
		t.Fatal("second run duplicated or overwrote project, membership or rating data")
	}
	if err := Reset(ctx, pool); err != nil {
		t.Fatal(err)
	}
	users, skills, userSkills, slots, profiles, cvs := counts()
	if users != 0 || userSkills != 0 || slots != 0 || profiles != 0 || cvs != 0 {
		t.Fatalf("reset left account data: %d users, %d user skills, %d slots, %d profiles, %d CVs", users, userSkills, slots, profiles, cvs)
	}
	var projectRows int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM projects) + (SELECT count(*) FROM project_roles)
		+ (SELECT count(*) FROM project_members) + (SELECT count(*) FROM ratings)`).Scan(&projectRows); err != nil {
		t.Fatal(err)
	}
	if projectRows != 0 {
		t.Fatalf("reset left %d project rows", projectRows)
	}
	f2, c2, p2 := starterLists()
	if f1 != f2 || c1 != c2 || p1 != p2 || skills != p2 {
		t.Fatalf("reset changed starter lists: %d,%d,%d then %d,%d,%d with %d skills", f1, c1, p1, f2, c2, p2, skills)
	}
}
