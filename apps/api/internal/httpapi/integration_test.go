package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	pdfapi "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"

	"github.com/thinc-org/CU-Profile/apps/api/internal/testdb"
)

type testAPI struct {
	t      *testing.T
	pool   *pgxpool.Pool
	server *httptest.Server
}

func newTestAPI(t *testing.T, demoSignIn bool) *testAPI {
	t.Helper()
	pool := testdb.New(t)
	server := httptest.NewServer(New(Options{DB: pool, DemoSignIn: demoSignIn}))
	t.Cleanup(server.Close)
	return &testAPI{t, pool, server}
}

func (a *testAPI) exec(sql string, args ...any) {
	a.t.Helper()
	if _, err := a.pool.Exec(context.Background(), sql, args...); err != nil {
		a.t.Fatalf("exec %q: %v", sql, err)
	}
}

func (a *testAPI) scalar(dst any, sql string, args ...any) {
	a.t.Helper()
	if err := a.pool.QueryRow(context.Background(), sql, args...).Scan(dst); err != nil {
		a.t.Fatalf("query %q: %v", sql, err)
	}
}

// testClient is one browser: its cookie jar holds the session.
type testClient struct {
	api    *testAPI
	http   *http.Client
	userID string
	email  string
	token  string
}

func (a *testAPI) anonymous() *testClient {
	jar, _ := cookiejar.New(nil)
	return &testClient{api: a, http: &http.Client{Jar: jar}}
}

func (a *testAPI) signIn(email string) *testClient {
	a.t.Helper()
	c := a.anonymous()
	response, body := c.send("POST", "/auth/demo-sign-in", map[string]string{"email": email})
	if response.StatusCode != http.StatusOK {
		a.t.Fatalf("sign in %s: %d %s", email, response.StatusCode, body)
	}
	var me struct {
		ID string `json:"id"`
	}
	mustDecode(a.t, body, &me)
	c.userID, c.email = me.ID, email
	for _, cookie := range response.Cookies() {
		if cookie.Name == sessionCookie {
			c.token = cookie.Value
		}
	}
	return c
}

func (c *testClient) send(method, path string, body any) (*http.Response, []byte) {
	c.api.t.Helper()
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			c.api.t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}
	request, err := http.NewRequest(method, c.api.server.URL+apiPrefix+path, reader)
	if err != nil {
		c.api.t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	return c.do(request)
}

func (c *testClient) do(request *http.Request) (*http.Response, []byte) {
	c.api.t.Helper()
	response, err := c.http.Do(request)
	if err != nil {
		c.api.t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		c.api.t.Fatal(err)
	}
	return response, body
}

// expect sends a request and fails the test on any other status.
func (c *testClient) expect(status int, method, path string, body any) []byte {
	c.api.t.Helper()
	response, raw := c.send(method, path, body)
	if response.StatusCode != status {
		c.api.t.Fatalf("%s %s: got %d, want %d: %s", method, path, response.StatusCode, status, raw)
	}
	return raw
}

func (c *testClient) upload(fileName string, content []byte) (*http.Response, []byte) {
	c.api.t.Helper()
	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	part, err := writer.CreateFormFile("file", fileName)
	if err != nil {
		c.api.t.Fatal(err)
	}
	part.Write(content)
	writer.Close()
	request, err := http.NewRequest("POST", c.api.server.URL+apiPrefix+"/me/cv", &form)
	if err != nil {
		c.api.t.Fatal(err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return c.do(request)
}

func mustDecode(t *testing.T, raw []byte, dst any) {
	t.Helper()
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
}

func errorCode(t *testing.T, raw []byte) string {
	t.Helper()
	var body struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	mustDecode(t, raw, &body)
	if body.Error.Message == "" {
		t.Fatalf("error has no message: %s", raw)
	}
	return body.Error.Code
}

func (a *testAPI) facultyID(name string) string {
	a.t.Helper()
	var id string
	a.scalar(&id, `SELECT id FROM faculties WHERE name = $1`, name)
	return id
}

// completeProfile makes this student searchable.
func (c *testClient) completeProfile(name string, skills ...string) {
	c.api.t.Helper()
	c.expect(http.StatusOK, "PUT", "/me/profile", map[string]any{
		"displayName": name, "facultyId": c.api.facultyID("Faculty of Engineering"), "skills": skills,
	})
}

// insertTeam writes a team led by lead, with one role holding the lead. A non-zero
// completedAt makes it a completed team, otherwise it's still recruiting.
func (a *testAPI) insertTeam(lead *testClient, name string, completedAt time.Time) (projectID, roleID string) {
	a.t.Helper()
	ctx := context.Background()
	var categoryID string
	a.scalar(&categoryID, `SELECT id FROM project_categories LIMIT 1`)
	var err error
	if completedAt.IsZero() {
		err = a.pool.QueryRow(ctx, `
			INSERT INTO projects (name, description, category_id, lead_user_id, expected_duration_value, expected_duration_unit)
			VALUES ($1, 'd', $2, $3, 1, 'month') RETURNING id`, name, categoryID, lead.userID).Scan(&projectID)
	} else {
		err = a.pool.QueryRow(ctx, `
			INSERT INTO projects (name, description, category_id, lead_user_id, expected_duration_value, expected_duration_unit,
				status, recruitment_status, expected_end_date, started_at, completed_at)
			VALUES ($1, 'd', $2, $3, 1, 'month', 'completed', 'closed', $4::timestamptz::date, $4::timestamptz - interval '30 days', $4::timestamptz)
			RETURNING id`, name, categoryID, lead.userID, completedAt).Scan(&projectID)
	}
	if err != nil {
		a.t.Fatal(err)
	}
	a.scalar(&roleID, `INSERT INTO project_roles (project_id, name, capacity) VALUES ($1, 'Member', 10) RETURNING id`, projectID)
	a.addMember(projectID, roleID, lead)
	return projectID, roleID
}

func (a *testAPI) addMember(projectID, roleID string, member *testClient) {
	a.t.Helper()
	a.exec(`INSERT INTO project_members (project_id, role_id, user_id, joined_at) SELECT $1, $2, $3, COALESCE(started_at,created_at) FROM projects WHERE id=$1`, projectID, roleID, member.userID)
}

const (
	aliceEmail = "6730000021@student.chula.ac.th"
	bobEmail   = "6730000121@student.chula.ac.th"
	caraEmail  = "6730000221@student.chula.ac.th"
)

func TestSessionCookieAndStoredHash(t *testing.T) {
	api := newTestAPI(t, true)
	alice := api.anonymous()
	response, _ := alice.send("POST", "/auth/demo-sign-in", map[string]string{"email": "  " + strings.ToUpper(aliceEmail)})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("sign in: %d", response.StatusCode)
	}
	var cookie *http.Cookie
	for _, c := range response.Cookies() {
		if c.Name == sessionCookie {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no session cookie")
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.Secure {
		t.Fatalf("cookie attributes: %+v", cookie)
	}
	if cookie.MaxAge != 30*24*60*60 {
		t.Fatalf("cookie max age %d", cookie.MaxAge)
	}
	var stored string
	var plainRows int
	api.scalar(&stored, `SELECT token_hash FROM sessions`)
	api.scalar(&plainRows, `SELECT count(*) FROM sessions WHERE token_hash = $1`, cookie.Value)
	if stored != hashToken(cookie.Value) || plainRows != 0 {
		t.Fatalf("stored %q is not the hash of the token", stored)
	}
	var subject string
	api.scalar(&subject, `SELECT oauth_subject FROM users`)
	if subject != "demo:"+aliceEmail {
		t.Fatalf("subject %q", subject)
	}
	var days float64
	api.scalar(&days, `SELECT extract(epoch FROM expires_at - created_at) / 86400 FROM sessions`)
	if days < 29.9 || days > 30.1 {
		t.Fatalf("session lasts %v days", days)
	}
	alice.expect(http.StatusOK, "GET", "/me", nil)
}

func TestSecureCookieWhenConfigured(t *testing.T) {
	pool := testdb.New(t)
	server := httptest.NewServer(New(Options{DB: pool, DemoSignIn: true, CookieSecure: true}))
	defer server.Close()
	response, err := http.Post(server.URL+apiPrefix+"/auth/demo-sign-in", "application/json", strings.NewReader(`{"email":"`+aliceEmail+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if cookies := response.Cookies(); len(cookies) != 1 || !cookies[0].Secure {
		t.Fatalf("cookies: %+v", cookies)
	}
}

func TestRevokedExpiredAndMissingSessionsAreRejected(t *testing.T) {
	api := newTestAPI(t, true)
	if code := errorCode(t, api.anonymous().expect(http.StatusUnauthorized, "GET", "/me", nil)); code != "not_signed_in" {
		t.Fatalf("code %q", code)
	}

	revoked := api.signIn(aliceEmail)
	api.exec(`UPDATE sessions SET revoked_at = now() WHERE token_hash = $1`, hashToken(revoked.token))
	revoked.expect(http.StatusUnauthorized, "GET", "/me", nil)

	expired := api.signIn(bobEmail)
	api.exec(`UPDATE sessions SET expires_at = now() - interval '1 second' WHERE token_hash = $1`, hashToken(expired.token))
	expired.expect(http.StatusUnauthorized, "GET", "/me", nil)

	// The stored hash itself must not work as a cookie.
	cara := api.signIn(caraEmail)
	forged := api.anonymous()
	request, _ := http.NewRequest("GET", api.server.URL+apiPrefix+"/me", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: hashToken(cara.token)})
	if response, _ := forged.do(request); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("hash used as token: %d", response.StatusCode)
	}
}

func TestSignOutRevokesOnlyThisSession(t *testing.T) {
	api := newTestAPI(t, true)
	laptop := api.signIn(aliceEmail)
	phone := api.signIn(aliceEmail)

	response, _ := laptop.send("POST", "/auth/sign-out", nil)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("sign out: %d", response.StatusCode)
	}
	if cookies := response.Cookies(); len(cookies) != 1 || cookies[0].MaxAge >= 0 || cookies[0].Value != "" {
		t.Fatalf("cookie not cleared: %+v", cookies)
	}
	// The old token stays dead even if a client kept it.
	request, _ := http.NewRequest("GET", api.server.URL+apiPrefix+"/me", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookie, Value: laptop.token})
	if response, _ := api.anonymous().do(request); response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("revoked token: %d", response.StatusCode)
	}
	phone.expect(http.StatusOK, "GET", "/me", nil)
}

func TestDemoSignInIsOffByDefault(t *testing.T) {
	api := newTestAPI(t, false)
	client := api.anonymous()
	client.expect(http.StatusNotFound, "POST", "/auth/demo-sign-in", map[string]string{"email": aliceEmail})
	client.expect(http.StatusNotFound, "GET", "/auth/demo-accounts", nil)
	var users int
	api.scalar(&users, `SELECT count(*) FROM users`)
	if users != 0 {
		t.Fatal("a user was created")
	}
}

func TestDemoSignInRejectsOtherEmails(t *testing.T) {
	api := newTestAPI(t, true)
	client := api.anonymous()
	for _, email := range []string{
		"", "someone@gmail.com", "someone@chula.ac.th", "@student.chula.ac.th",
		"someone@student.chula.ac.th.evil.com", "someone@evil.com@student.chula.ac.th",
		"some one@student.chula.ac.th", "someone@studentxchula.ac.th",
	} {
		raw := client.expect(http.StatusBadRequest, "POST", "/auth/demo-sign-in", map[string]string{"email": email})
		if code := errorCode(t, raw); code != "not_chula_email" {
			t.Errorf("%q: code %q", email, code)
		}
	}
	var users int
	api.scalar(&users, `SELECT count(*) FROM users`)
	if users != 0 {
		t.Fatalf("%d users created", users)
	}
}

func TestEmailNeverInAnotherStudentsResponses(t *testing.T) {
	api := newTestAPI(t, true)
	alice := api.signIn(aliceEmail)
	alice.completeProfile("Alice", "Go")
	bob := api.signIn(bobEmail)
	bob.completeProfile("Bob", "Go")
	projectID, roleID := api.insertTeam(alice, "Done team", time.Now().Add(-30*24*time.Hour))
	api.addMember(projectID, roleID, bob)
	api.exec(`INSERT INTO ratings (project_id, reviewer_id, recipient_id, score) VALUES ($1, $2, $3, 5)`, projectID, alice.userID, bob.userID)

	for _, path := range []string{
		"/profiles", "/profiles?skill=go", "/profiles?q=Alice", "/profiles/" + alice.userID, "/profiles/" + alice.userID + "/history",
		"/profiles/" + alice.userID + "/ratings", "/profiles/" + alice.userID + "/teammates",
		"/profiles/" + bob.userID + "/ratings", "/profiles/" + bob.userID + "/teammates",
	} {
		raw := bob.expect(http.StatusOK, "GET", path, nil)
		if strings.Contains(string(raw), aliceEmail) || strings.Contains(string(raw), "6730000021") {
			t.Errorf("%s leaks the email: %s", path, raw)
		}
	}
	// Bob sees his own, which also proves the check above is looking at the right kind of value.
	if raw := bob.expect(http.StatusOK, "GET", "/me/profile", nil); !strings.Contains(string(raw), bobEmail) {
		t.Errorf("own profile has no email: %s", raw)
	}
}

func validPDF() []byte { return pdfFixture("1.4", 0, "Catalog", "Page") }

// pdfFixture writes a one-page PDF with real cross-reference offsets and a page tree.
func pdfFixture(version string, padding int, catalogType, pageType string) []byte {
	stream := "BT /F1 12 Tf 72 720 Td (Student CV) Tj ET\n"
	resources := "<< /Font << /F1 5 0 R >> >>"
	if version == "2.0" {
		// PDF 2.0 requires descriptors for all fonts. Vector content needs no font.
		stream, resources = "0 0 0 rg 72 720 100 2 re f\n", "<< >>"
	}
	stream += strings.Repeat(" ", padding)
	objects := []string{
		fmt.Sprintf("<< /Type /%s /Pages 2 0 R >>", catalogType),
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		fmt.Sprintf("<< /Type /%s /Parent 2 0 R /MediaBox [0 0 612 792] /Resources %s /Contents 4 0 R >>", pageType, resources),
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream),
	}
	if version != "2.0" {
		objects = append(objects, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "%%PDF-%s\n", version)
	offsets := make([]int, len(objects))
	for i, object := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return out.Bytes()
}

func pdfFixtureAtSize(size int) []byte {
	padding := size - len(validPDF())
	for {
		content := pdfFixture("1.4", padding, "Catalog", "Page")
		if len(content) == size {
			return content
		}
		padding += size - len(content)
	}
}

type cvView struct {
	CV struct {
		Available bool    `json:"available"`
		FileName  *string `json:"fileName"`
	} `json:"cv"`
}

func TestCVPrivacy(t *testing.T) {
	api := newTestAPI(t, true)
	owner := api.signIn(aliceEmail)
	owner.completeProfile("Alice", "Go")
	viewer := api.signIn(bobEmail)
	link := "/users/" + owner.userID + "/cv"
	profile := "/profiles/" + owner.userID

	if code := errorCode(t, viewer.expect(http.StatusNotFound, "GET", link, nil)); code != "cv_not_found" {
		t.Fatalf("no cv: %q", code)
	}
	if response, raw := owner.upload("my cv.pdf", validPDF()); response.StatusCode != http.StatusOK {
		t.Fatalf("upload: %d %s", response.StatusCode, raw)
	}
	var key string
	api.scalar(&key, `SELECT cv_storage_key FROM profiles WHERE user_id = $1`, owner.userID)
	if key != "db:"+owner.userID {
		t.Fatalf("storage key %q", key)
	}

	checkHidden := func(when string) {
		t.Helper()
		raw := viewer.expect(http.StatusForbidden, "GET", link, nil)
		if code := errorCode(t, raw); code != "cv_not_shared" {
			t.Fatalf("%s: code %q", when, code)
		}
		if strings.Contains(string(raw), "%PDF") {
			t.Fatalf("%s: response has the file", when)
		}
		var view cvView
		mustDecode(t, viewer.expect(http.StatusOK, "GET", profile, nil), &view)
		if view.CV.Available || view.CV.FileName != nil {
			t.Fatalf("%s: profile exposes the cv: %+v", when, view.CV)
		}
	}
	checkHidden("private by default")
	if raw := owner.expect(http.StatusOK, "GET", link, nil); !bytes.Equal(raw, validPDF()) {
		t.Fatal("owner got different bytes")
	}

	owner.expect(http.StatusOK, "PUT", "/me/cv/sharing", map[string]bool{"shared": true})
	response, raw := viewer.send("GET", link, nil)
	if response.StatusCode != http.StatusOK || !bytes.Equal(raw, validPDF()) {
		t.Fatalf("shared: %d", response.StatusCode)
	}
	if got := response.Header.Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("content type %q", got)
	}
	if got := response.Header.Get("Content-Disposition"); !strings.HasPrefix(got, "inline") || !strings.Contains(got, "my cv.pdf") {
		t.Fatalf("disposition %q", got)
	}
	if got := response.Header.Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Fatalf("cached copies could outlive revocation: %q", got)
	}
	var view cvView
	mustDecode(t, viewer.expect(http.StatusOK, "GET", profile, nil), &view)
	if !view.CV.Available || view.CV.FileName == nil || *view.CV.FileName != "my cv.pdf" {
		t.Fatalf("shared profile: %+v", view.CV)
	}

	// Revoking blocks the same link that worked a moment ago.
	owner.expect(http.StatusOK, "PUT", "/me/cv/sharing", map[string]bool{"shared": false})
	checkHidden("after revoking")

	// Sharing doesn't survive deleting the file: the next upload starts private.
	owner.expect(http.StatusOK, "PUT", "/me/cv/sharing", map[string]bool{"shared": true})
	owner.expect(http.StatusNoContent, "DELETE", "/me/cv", nil)
	viewer.expect(http.StatusNotFound, "GET", link, nil)
	var files int
	api.scalar(&files, `SELECT count(*) FROM cv_files`)
	if files != 0 {
		t.Fatal("file bytes survived delete")
	}
	owner.upload("again.pdf", validPDF())
	checkHidden("after deleting and uploading again")

	api.anonymous().expect(http.StatusUnauthorized, "GET", link, nil)

	// A closed account's CV is gone, even if it was shared.
	owner.expect(http.StatusOK, "PUT", "/me/cv/sharing", map[string]bool{"shared": true})
	owner.expect(http.StatusNoContent, "POST", "/me/close", map[string]any{})
	viewer.expect(http.StatusNotFound, "GET", link, nil)
}

func TestInvalidCVIsRejected(t *testing.T) {
	api := newTestAPI(t, true)
	alice := api.signIn(aliceEmail)
	pad := func(size int) []byte {
		content := bytes.Repeat([]byte("x"), size)
		copy(content, "%PDF-1.4\n")
		copy(content[size-6:], "%%EOF\n")
		return content
	}
	for name, tc := range map[string]struct {
		content []byte
		status  int
		code    string
	}{
		"text named pdf":     {[]byte("hello, not a pdf"), http.StatusBadRequest, "not_pdf"},
		"empty":              {nil, http.StatusBadRequest, "not_pdf"},
		"header only":        {[]byte("%PDF-1.4\nno trailer"), http.StatusBadRequest, "not_pdf"},
		"trailer only":       {[]byte("hello\n%%EOF\n"), http.StatusBadRequest, "not_pdf"},
		"bad version":        {[]byte("%PDF-\n%%EOF\n"), http.StatusBadRequest, "not_pdf"},
		"header not first":   {[]byte("<html>%PDF-1.4\n%%EOF"), http.StatusBadRequest, "not_pdf"},
		"one byte too large": {pad(maxCVBytes + 1), http.StatusRequestEntityTooLarge, "cv_too_large"},
		"far too large":      {pad(maxCVBytes * 2), http.StatusRequestEntityTooLarge, "cv_too_large"},
	} {
		response, raw := alice.upload("cv.pdf", tc.content)
		if response.StatusCode != tc.status || errorCode(t, raw) != tc.code {
			t.Errorf("%s: %d %s", name, response.StatusCode, raw)
		}
	}
	var files int
	api.scalar(&files, `SELECT count(*) FROM cv_files`)
	if files != 0 {
		t.Fatalf("%d rejected files were stored", files)
	}
	if response, raw := alice.upload("cv.pdf", pdfFixtureAtSize(maxCVBytes)); response.StatusCode != http.StatusOK {
		t.Fatalf("exactly 5 MB: %d %s", response.StatusCode, raw)
	}
	// A JSON body can't stand in for the upload.
	alice.expect(http.StatusBadRequest, "POST", "/me/cv", map[string]string{"file": "x"})
}

func TestPDFStructureValidationPreservesExistingCV(t *testing.T) {
	api := newTestAPI(t, true)
	owner := api.signIn(aliceEmail)
	for _, version := range []string{"1.4", "1.7", "2.0"} {
		content := pdfFixture(version, 0, "Catalog", "Page")
		if response, raw := owner.upload("version.pdf", content); response.StatusCode != http.StatusOK {
			t.Fatalf("PDF %s: %d %s", version, response.StatusCode, raw)
		}
		if stored := owner.expect(http.StatusOK, "GET", "/users/"+owner.userID+"/cv", nil); !bytes.Equal(stored, content) {
			t.Fatalf("PDF %s bytes changed", version)
		}
	}
	modernConf := model.NewStatelessConfiguration()
	modernConf.WriteXRefStream, modernConf.WriteObjectStream = true, true
	var modern bytes.Buffer
	if err := pdfapi.Optimize(context.Background(), bytes.NewReader(pdfFixture("1.7", 0, "Catalog", "Page")), &modern, modernConf, nil); err != nil {
		t.Fatalf("build modern fixture: %v", err)
	}
	parsed, err := pdfapi.ReadContext(context.Background(), bytes.NewReader(modern.Bytes()), modernConf)
	if err != nil || !parsed.Read.UsingXRefStreams || !parsed.Read.UsingObjectStreams {
		t.Fatalf("modern fixture must contain xref and object streams: %v", err)
	}
	if response, raw := owner.upload("modern.pdf", modern.Bytes()); response.StatusCode != http.StatusOK {
		t.Fatalf("modern PDF: %d %s", response.StatusCode, raw)
	}
	if stored := owner.expect(http.StatusOK, "GET", "/users/"+owner.userID+"/cv", nil); !bytes.Equal(stored, modern.Bytes()) {
		t.Fatal("modern PDF bytes changed")
	}
	original := validPDF()
	if response, raw := owner.upload("original.pdf", original); response.StatusCode != http.StatusOK {
		t.Fatalf("original upload: %d %s", response.StatusCode, raw)
	}
	owner.expect(http.StatusOK, "PUT", "/me/cv/sharing", map[string]bool{"shared": true})
	var uploadedAt time.Time
	api.scalar(&uploadedAt, `SELECT cv_uploaded_at FROM profiles WHERE user_id=$1`, owner.userID)

	conf := model.NewStatelessConfiguration()
	conf.UserPW, conf.OwnerPW = "test-user-password", "test-owner-password"
	var encrypted bytes.Buffer
	if err := pdfapi.Encrypt(context.Background(), bytes.NewReader(original), &encrypted, conf); err != nil {
		t.Fatalf("encrypt fixture: %v", err)
	}
	for name, content := range map[string][]byte{
		"markers without objects": []byte("%PDF-1.4\nbody\n%%EOF\n"),
		"broken catalog":          pdfFixture("1.4", 0, "Invalid", "Page"),
		"broken page tree":        pdfFixture("1.4", 0, "Catalog", "Invalid"),
		"missing xref":            bytes.Replace(original, []byte("xref\n"), []byte("xxxx\n"), 1),
		"invalid xref offset":     bytes.Replace(original, []byte(fmt.Sprintf("startxref\n%d\n", bytes.Index(original, []byte("xref\n")))), []byte("startxref\n0\n"), 1),
		"password required":       encrypted.Bytes(),
	} {
		t.Run(name, func(t *testing.T) {
			response, raw := owner.upload("invalid.pdf", content)
			if response.StatusCode != http.StatusBadRequest || errorCode(t, raw) != "not_pdf" {
				t.Fatalf("got %d %s, want 400 not_pdf", response.StatusCode, raw)
			}
			var stored []byte
			var fileName string
			var size int
			var shared bool
			var savedAt time.Time
			api.scalar(&stored, `SELECT content FROM cv_files WHERE user_id=$1`, owner.userID)
			if err := api.pool.QueryRow(context.Background(), `SELECT cv_file_name,cv_size_bytes,cv_shared,cv_uploaded_at FROM profiles WHERE user_id=$1`, owner.userID).Scan(&fileName, &size, &shared, &savedAt); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(stored, original) || fileName != "original.pdf" || size != len(original) || !shared || !savedAt.Equal(uploadedAt) {
				t.Fatal("invalid replacement changed the stored CV or its metadata")
			}
		})
	}
}

func TestDirectoryListsOnlySearchableProfiles(t *testing.T) {
	api := newTestAPI(t, true)
	viewer := api.signIn(aliceEmail)
	faculty := api.facultyID("Faculty of Science")

	put := func(email string, body map[string]any) *testClient {
		c := api.signIn(email)
		c.expect(http.StatusOK, "PUT", "/me/profile", body)
		return c
	}
	complete := put("6730001021@student.chula.ac.th", map[string]any{"displayName": "Complete", "facultyId": faculty, "skills": []string{"Excel", "Go", "Python", "SQL", "Sales", "Figma", "React Native"}})
	put("6730001121@student.chula.ac.th", map[string]any{"facultyId": faculty, "skills": []string{"Go"}})
	put("6730001221@student.chula.ac.th", map[string]any{"displayName": "No faculty", "skills": []string{"Go"}})
	noSkill := put("6730001321@student.chula.ac.th", map[string]any{"displayName": "No skill", "facultyId": faculty, "skills": []string{}})
	put("6730001421@student.chula.ac.th", map[string]any{"displayName": "   ", "facultyId": faculty, "skills": []string{"Go"}})
	closing := put("6730001521@student.chula.ac.th", map[string]any{"displayName": "Closing", "facultyId": faculty, "skills": []string{"Go"}})

	type page struct {
		Items []struct {
			UserID string   `json:"userId"`
			Name   string   `json:"displayName"`
			Skills []string `json:"skills"`
		} `json:"items"`
		Total int `json:"total"`
	}
	list := func(query string) page {
		t.Helper()
		var out page
		mustDecode(t, viewer.expect(http.StatusOK, "GET", "/profiles"+query, nil), &out)
		return out
	}
	names := func(p page) string {
		found := []string{}
		for _, item := range p.Items {
			found = append(found, item.Name)
		}
		return strings.Join(found, ",")
	}
	if got := names(list("")); got != "Closing,Complete" {
		t.Fatalf("directory: %q", got)
	}
	closing.expect(http.StatusNoContent, "POST", "/me/close", map[string]any{})
	if got := list(""); names(got) != "Complete" || got.Total != 1 {
		t.Fatalf("after closing: %+v", got)
	}

	// A name match cannot bypass the same visibility gate as a skill match.
	for _, query := range []string{"?q=Closing", "?q=No", "?q=6730001021"} {
		if got := list(query); got.Total != 0 || len(got.Items) != 0 {
			t.Fatalf("name search exposed a hidden profile or searched email: %s: %+v", query, got)
		}
	}
	if got := list("?q=Go"); names(got) != "Complete" || got.Total != 1 {
		t.Fatalf("name-or-skill search exposed an incomplete or closed profile: %+v", got)
	}

	// Partial, case-insensitive, and the match leads the capped skill list.
	found := list("?skill=REACT")
	if names(found) != "Complete" || found.Items[0].Skills[0] != "React Native" || len(found.Items[0].Skills) != cardSkillLimit {
		t.Fatalf("skill search: %+v", found)
	}
	if got := list("?skill=%25"); got.Total != 0 || len(got.Items) != 0 {
		t.Fatalf("%% must be literal: %+v", got)
	}
	if raw := viewer.expect(http.StatusOK, "GET", "/profiles?skill=nothing", nil); !strings.Contains(string(raw), `"items":[]`) {
		t.Fatalf("empty page: %s", raw)
	}

	me := func(c *testClient) (complete bool) {
		t.Helper()
		var out struct {
			ProfileComplete bool `json:"profileComplete"`
		}
		mustDecode(t, c.expect(http.StatusOK, "GET", "/me", nil), &out)
		return out.ProfileComplete
	}
	if !me(complete) || me(noSkill) || me(viewer) {
		t.Fatal("profileComplete disagrees with the directory")
	}
	// A profile hidden from search stays openable if it has a name, but not without one.
	viewer.expect(http.StatusOK, "GET", "/profiles/"+noSkill.userID, nil)
	viewer.expect(http.StatusNotFound, "GET", "/profiles/"+viewer.userID, nil)
	viewer.expect(http.StatusNotFound, "GET", "/profiles/not-a-uuid", nil)
}

func TestSkillsMatchExistingOrBecomeCustom(t *testing.T) {
	api := newTestAPI(t, true)
	alice := api.signIn(aliceEmail)
	var before int
	api.scalar(&before, `SELECT count(*) FROM skills`)
	raw := alice.expect(http.StatusOK, "PUT", "/me/profile", map[string]any{"skills": []string{"  figma ", "Figma", "Basket  weaving"}})
	var profile struct {
		Skills []string `json:"skills"`
	}
	mustDecode(t, raw, &profile)
	if strings.Join(profile.Skills, "|") != "Figma|Basket weaving" {
		t.Fatalf("skills: %v", profile.Skills)
	}
	var after int
	var predefined bool
	api.scalar(&after, `SELECT count(*) FROM skills`)
	api.scalar(&predefined, `SELECT is_predefined FROM skills WHERE normalized_name = 'basket weaving'`)
	if after != before+1 || predefined {
		t.Fatalf("custom skill: %d then %d, predefined %v", before, after, predefined)
	}
}

func TestMutationsCheckBodyRules(t *testing.T) {
	api := newTestAPI(t, true)
	alice := api.signIn(aliceEmail)

	raw := alice.expect(http.StatusBadRequest, "PUT", "/me/profile", map[string]any{"isAdmin": true})
	if code := errorCode(t, raw); code != "invalid_json" {
		t.Fatalf("unknown field: %q", code)
	}
	var fields struct {
		Error struct {
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	mustDecode(t, alice.expect(http.StatusBadRequest, "PUT", "/me/profile", map[string]any{"year": 9}), &fields)
	if fields.Error.Fields["year"] == "" {
		t.Fatalf("no field error: %+v", fields)
	}
	alice.expect(http.StatusRequestEntityTooLarge, "PUT", "/me/profile", map[string]any{"headline": strings.Repeat("x", maxJSONBody)})

	// A cross-site form can only send these content types, so none of them may mutate.
	for _, contentType := range []string{"", "text/plain", "application/x-www-form-urlencoded", "multipart/form-data; boundary=x"} {
		request, _ := http.NewRequest("POST", api.server.URL+apiPrefix+"/me/close", strings.NewReader("{}"))
		if contentType != "" {
			request.Header.Set("Content-Type", contentType)
		}
		if response, _ := alice.do(request); response.StatusCode != http.StatusBadRequest {
			t.Errorf("%q: %d", contentType, response.StatusCode)
		}
	}
	alice.expect(http.StatusOK, "GET", "/me", nil)

	for _, start := range []string{"09:15", "24:00", "9:00", "09:00:00", "bad"} {
		alice.expect(http.StatusBadRequest, "PUT", "/me/availability", map[string]any{"slots": []map[string]any{{"day": 1, "start": start}}})
	}
	alice.expect(http.StatusBadRequest, "PUT", "/me/availability", map[string]any{"slots": []map[string]any{{"day": 8, "start": "09:00"}}})
	var saved struct {
		Slots []Slot `json:"slots"`
	}
	mustDecode(t, alice.expect(http.StatusOK, "PUT", "/me/availability", map[string]any{"slots": []map[string]any{
		{"day": 2, "start": "09:30"}, {"day": 1, "start": "23:30"}, {"day": 2, "start": "09:30"},
	}}), &saved)
	if len(saved.Slots) != 2 || saved.Slots[0] != (Slot{1, "23:30"}) {
		t.Fatalf("slots: %+v", saved.Slots)
	}
	mustDecode(t, alice.expect(http.StatusOK, "PUT", "/me/availability", map[string]any{"slots": []any{}}), &saved)
	if len(saved.Slots) != 0 {
		t.Fatalf("replace with none left %+v", saved.Slots)
	}
}

func TestEmptyBodyMutationsRejectInvalidJSONWithoutChangingData(t *testing.T) {
	for _, action := range []string{"delete role", "delete CV", "sign out", "read notification", "read all notifications"} {
		t.Run(action, func(t *testing.T) {
			api := newTestAPI(t, true)
			owner := api.student(aliceEmail, "Owner")
			applicant := api.student(bobEmail, "Applicant")
			project := api.postProject(owner, "Keep this team", map[string]int{"Developer": 1})
			roleID := project.role(t, "Developer").ID
			application := applicant.applyTo(project, "Developer")
			if response, raw := owner.upload("keep.pdf", validPDF()); response.StatusCode != http.StatusOK {
				t.Fatalf("upload: %d %s", response.StatusCode, raw)
			}
			owner.expect(http.StatusOK, "PUT", "/me/cv/sharing", map[string]bool{"shared": true})
			var notificationID string
			api.scalar(&notificationID, `SELECT id FROM notifications WHERE user_id=$1 LIMIT 1`, owner.userID)
			api.exec(`INSERT INTO notifications(user_id,type) VALUES($1,'member_left')`, owner.userID)
			var unreadBefore int
			api.scalar(&unreadBefore, `SELECT count(*) FROM notifications WHERE user_id=$1 AND read_at IS NULL`, owner.userID)

			method, path := "POST", "/auth/sign-out"
			switch action {
			case "delete role":
				method, path = "DELETE", "/projects/"+project.ID+"/roles/"+roleID
			case "delete CV":
				method, path = "DELETE", "/me/cv"
			case "read notification":
				path = "/notifications/" + notificationID + "/read"
			case "read all notifications":
				path = "/notifications/read-all"
			}
			for _, body := range []struct {
				name, raw, code string
				status          int
			}{
				{"malformed", "{", "invalid_json", http.StatusBadRequest},
				{"unknown field", `{"unexpected":true}`, "invalid_json", http.StatusBadRequest},
				{"oversized", strings.Repeat(" ", maxJSONBody) + "{}", "body_too_large", http.StatusRequestEntityTooLarge},
			} {
				t.Run(body.name, func(t *testing.T) {
					response, raw := owner.rawCall(method, path, body.raw)()
					if response.StatusCode != body.status || errorCode(t, raw) != body.code {
						t.Fatalf("got %d %s, want %d %s", response.StatusCode, raw, body.status, body.code)
					}
					var removed, revoked bool
					api.scalar(&removed, `SELECT removed_at IS NOT NULL FROM project_roles WHERE id=$1`, roleID)
					api.scalar(&revoked, `SELECT revoked_at IS NOT NULL FROM sessions WHERE token_hash=$1`, hashToken(owner.token))
					status, _ := api.requestState(application.ID)
					var stored []byte
					api.scalar(&stored, `SELECT content FROM cv_files WHERE user_id=$1`, owner.userID)
					var shared bool
					api.scalar(&shared, `SELECT cv_shared FROM profiles WHERE user_id=$1`, owner.userID)
					var unread int
					api.scalar(&unread, `SELECT count(*) FROM notifications WHERE user_id=$1 AND read_at IS NULL`, owner.userID)
					if removed || revoked || status != "pending" || !bytes.Equal(stored, validPDF()) || !shared || unread != unreadBefore || len(response.Cookies()) != 0 {
						t.Fatalf("rejected body changed data: role removed=%v, session revoked=%v, request=%s, CV preserved=%v, shared=%v, unread=%d/%d, cookies=%v",
							removed, revoked, status, bytes.Equal(stored, validPDF()), shared, unread, unreadBefore, response.Cookies())
					}
				})
			}
		})
	}
}

func TestRatingsPublishOnlyAfterTheWindow(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.signIn(aliceEmail)
	lead.completeProfile("Lead", "Go")
	student := api.signIn(bobEmail)
	student.completeProfile("Student", "Go")
	former := api.signIn(caraEmail)
	former.completeProfile("Former", "Go")

	closedTeam, closedRole := api.insertTeam(lead, "Closed window", time.Now().Add(-15*24*time.Hour))
	api.addMember(closedTeam, closedRole, student)
	openTeam, openRole := api.insertTeam(lead, "Open window", time.Now().Add(-13*24*time.Hour))
	api.addMember(openTeam, openRole, student)
	// Someone who left before completion remains a past teammate but cannot have a score.
	api.exec(`INSERT INTO project_members (project_id, role_id, user_id, joined_at, left_at, left_reason) VALUES ($1, $2, $3, now()-interval '40 days', now()-interval '20 days', 'left')`, closedTeam, closedRole, former.userID)

	api.exec(`INSERT INTO ratings (project_id, reviewer_id, recipient_id, score) VALUES ($1, $2, $3, 4), ($4, $2, $3, 1)`,
		closedTeam, lead.userID, student.userID, openTeam)

	var ratings struct {
		Rating    Rating `json:"rating"`
		Published []struct {
			Score   int    `json:"score"`
			Project idName `json:"project"`
		} `json:"published"`
		PendingProjects []idName `json:"pendingProjects"`
	}
	viewer := former
	mustDecode(t, viewer.expect(http.StatusOK, "GET", "/profiles/"+student.userID+"/ratings", nil), &ratings)
	if ratings.Rating.Count != 1 || ratings.Rating.Average == nil || *ratings.Rating.Average != 4 {
		t.Fatalf("average must ignore the open window: %+v", ratings.Rating)
	}
	if len(ratings.Published) != 1 || ratings.Published[0].Project.Name != "Closed window" {
		t.Fatalf("published: %+v", ratings.Published)
	}
	if len(ratings.PendingProjects) != 1 || ratings.PendingProjects[0].Name != "Open window" {
		t.Fatalf("pending: %+v", ratings.PendingProjects)
	}

	var teammates struct {
		Items []struct {
			Teammate struct {
				ID string `json:"id"`
			} `json:"teammate"`
			Project idName `json:"project"`
			State   string `json:"state"`
			Score   *int   `json:"score"`
		} `json:"items"`
	}
	mustDecode(t, viewer.expect(http.StatusOK, "GET", "/profiles/"+student.userID+"/teammates", nil), &teammates)
	if len(teammates.Items) != 3 {
		t.Fatalf("teammates: %+v", teammates)
	}
	for _, item := range teammates.Items {
		if item.Teammate.ID == former.userID {
			if item.State != "not_rated" || item.Score != nil {
				t.Fatalf("former member has rating state: %+v", item)
			}
			continue
		}
		if item.Teammate.ID != lead.userID {
			t.Fatalf("unexpected teammate: %+v", item)
		}
		switch item.Project.Name {
		case "Open window":
			if item.State != "pending" || item.Score != nil {
				t.Fatalf("open window leaks a score: %+v", item)
			}
		case "Closed window":
			if item.State != "published" || item.Score == nil || *item.Score != 4 {
				t.Fatalf("closed window: %+v", item)
			}
		}
	}

	// The lead gave scores but received none, so the same window reads as not rated.
	mustDecode(t, viewer.expect(http.StatusOK, "GET", "/profiles/"+lead.userID+"/teammates", nil), &teammates)
	for _, item := range teammates.Items {
		if item.Project.Name == "Closed window" && (item.State != "not_rated" || item.Score != nil) {
			t.Fatalf("missing score: %+v", item)
		}
	}

	// Scores from a closed account stop counting at once.
	api.exec(`UPDATE users SET closed_at = now() WHERE id = $1`, lead.userID)
	mustDecode(t, viewer.expect(http.StatusOK, "GET", "/profiles/"+student.userID+"/ratings", nil), &ratings)
	if ratings.Rating.Count != 0 || ratings.Rating.Average != nil || len(ratings.Published) != 0 {
		t.Fatalf("closed reviewer still counted: %+v", ratings)
	}
}

func TestHistoryClassifiesMemberships(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.signIn(aliceEmail)
	lead.completeProfile("Lead", "Go")
	student := api.signIn(bobEmail)
	student.completeProfile("Student", "Go")

	live, liveRole := api.insertTeam(lead, "Live", time.Time{})
	api.addMember(live, liveRole, student)
	left, leftRole := api.insertTeam(lead, "Left", time.Time{})
	api.exec(`INSERT INTO project_members (project_id, role_id, user_id, left_at, left_reason) VALUES ($1, $2, $3, now(), 'removed')`, left, leftRole, student.userID)
	done, doneRole := api.insertTeam(lead, "Done", time.Now().Add(-time.Hour))
	api.addMember(done, doneRole, student)
	gone, goneRole := api.insertTeam(lead, "Gone", time.Time{})
	api.addMember(gone, goneRole, student)
	api.exec(`UPDATE projects SET deleted_at = now(), recruitment_status = 'closed' WHERE id = $1`, gone)

	var history struct {
		Current, Past []struct {
			Project struct {
				Name string `json:"name"`
			} `json:"project"`
			RoleName   string     `json:"roleName"`
			LeftReason *string    `json:"leftReason"`
			EndedAt    *time.Time `json:"endedAt"`
		}
	}
	mustDecode(t, lead.expect(http.StatusOK, "GET", "/profiles/"+student.userID+"/history", nil), &history)
	if len(history.Current) != 1 || history.Current[0].Project.Name != "Live" || history.Current[0].EndedAt != nil {
		t.Fatalf("current: %+v", history.Current)
	}
	if len(history.Past) != 2 {
		t.Fatalf("past: %+v", history.Past)
	}
	for _, item := range history.Past {
		if item.EndedAt == nil {
			t.Fatalf("no end date: %+v", item)
		}
		if item.Project.Name == "Left" && (item.LeftReason == nil || *item.LeftReason != "removed") {
			t.Fatalf("left reason: %+v", item)
		}
	}
}

func TestCloseAccount(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.signIn(aliceEmail)
	lead.completeProfile("Lead", "Go")
	closer := api.signIn(bobEmail)
	closer.completeProfile("Closer", "Go")

	joined, joinedRole := api.insertTeam(lead, "Joined", time.Time{})
	api.addMember(joined, joinedRole, closer)
	api.exec(`INSERT INTO leadership_transfers (project_id, from_user_id, to_user_id) VALUES ($1, $2, $3)`, joined, lead.userID, closer.userID)
	applied, appliedRole := api.insertTeam(lead, "Applied", time.Time{})
	var requestID string
	api.scalar(&requestID, `INSERT INTO join_requests (type, project_id, role_id, student_id) VALUES ('application', $1, $2, $3) RETURNING id`, applied, appliedRole, closer.userID)
	ended, endedRole := api.insertTeam(lead, "Ended", time.Now().Add(-20*24*time.Hour))
	api.addMember(ended, endedRole, closer)

	// Leading a team that hasn't ended blocks closing, and nothing changes.
	owned, _ := api.insertTeam(closer, "Owned", time.Time{})
	raw := closer.expect(http.StatusConflict, "POST", "/me/close", map[string]any{})
	if code := errorCode(t, raw); code != "must_transfer_or_delete" {
		t.Fatalf("code %q", code)
	}
	var pending int
	api.scalar(&pending, `SELECT count(*) FROM join_requests WHERE status = 'pending'`)
	if pending != 1 {
		t.Fatal("a refused close changed requests")
	}
	closer.expect(http.StatusOK, "GET", "/me", nil)
	api.exec(`UPDATE projects SET deleted_at = now(), recruitment_status = 'closed' WHERE id = $1`, owned)

	response, _ := closer.send("POST", "/me/close", map[string]any{})
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("close: %d", response.StatusCode)
	}
	if cookies := response.Cookies(); len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("cookie not cleared: %+v", cookies)
	}

	var status, reason string
	api.scalar(&status, `SELECT status::text FROM join_requests WHERE id = $1`, requestID)
	api.scalar(&reason, `SELECT cancel_reason::text FROM join_requests WHERE id = $1`, requestID)
	if status != "cancelled" || reason != "account_closed" {
		t.Fatalf("request: %s, %s", status, reason)
	}
	api.scalar(&reason, `SELECT left_reason::text FROM project_members WHERE project_id = $1 AND user_id = $2`, joined, closer.userID)
	if reason != "account_closed" {
		t.Fatalf("membership: %s", reason)
	}
	var stillOnEnded int
	api.scalar(&stillOnEnded, `SELECT count(*) FROM project_members WHERE project_id = $1 AND user_id = $2 AND left_at IS NULL`, ended, closer.userID)
	if stillOnEnded != 1 {
		t.Fatal("membership on an ended team must stay as it was")
	}
	api.scalar(&status, `SELECT status::text FROM leadership_transfers`)
	if status != "cancelled" {
		t.Fatalf("transfer: %s", status)
	}
	var live int
	api.scalar(&live, `SELECT count(*) FROM sessions WHERE user_id = $1 AND revoked_at IS NULL`, closer.userID)
	if live != 0 {
		t.Fatalf("%d sessions still live", live)
	}

	// Gone at once: no session, no sign-in, no profile, no directory card.
	closer.expect(http.StatusUnauthorized, "GET", "/me", nil)
	raw = api.anonymous().expect(http.StatusForbidden, "POST", "/auth/demo-sign-in", map[string]string{"email": bobEmail})
	if code := errorCode(t, raw); code != "account_closed" {
		t.Fatalf("sign in after close: %q", code)
	}
	for _, path := range []string{"", "/history", "/ratings", "/teammates"} {
		lead.expect(http.StatusNotFound, "GET", "/profiles/"+closer.userID+path, nil)
	}
	if raw := lead.expect(http.StatusOK, "GET", "/profiles", nil); strings.Contains(string(raw), closer.userID) {
		t.Fatalf("closed profile in directory: %s", raw)
	}

	// The lead hears about each change, written with the close itself.
	var inbox struct {
		Items []struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Link    string `json:"link"`
		} `json:"items"`
		UnreadCount int `json:"unreadCount"`
	}
	mustDecode(t, lead.expect(http.StatusOK, "GET", "/notifications", nil), &inbox)
	seen := map[string]string{}
	for _, item := range inbox.Items {
		seen[item.Type] = item.Link
		if item.Message == "" || strings.Contains(item.Message, "{") || strings.Contains(item.Message, "Closer") {
			t.Fatalf("message: %q", item.Message)
		}
	}
	if inbox.UnreadCount != 3 || seen["request_cancelled"] != "/requests/"+requestID ||
		seen["member_left"] != "/projects/"+joined || seen["transfer_cancelled"] != "/projects/"+joined {
		t.Fatalf("inbox: %+v", inbox)
	}
}

func TestNotificationsBelongToTheirOwner(t *testing.T) {
	api := newTestAPI(t, true)
	alice := api.signIn(aliceEmail)
	bob := api.signIn(bobEmail)
	projectID, _ := api.insertTeam(alice, "Team", time.Time{})

	insert := func(user *testClient, notificationType string, data notificationData) string {
		t.Helper()
		ctx := context.Background()
		tx, err := api.pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if err := notify(ctx, tx, user.userID, notificationType, projectID, "", data); err != nil {
			t.Fatal(err)
		}
		if err := tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		var id string
		api.scalar(&id, `SELECT id FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`, user.userID)
		return id
	}
	aliceNote := insert(alice, notifMemberLeft, notificationData{"actorName": "Bob"})
	bobNote := insert(bob, notifRateTeammates, nil)

	// Someone else's notification doesn't exist as far as Bob can tell.
	bob.expect(http.StatusNotFound, "POST", "/notifications/"+aliceNote+"/read", nil)
	bob.expect(http.StatusNotFound, "POST", "/notifications/not-a-uuid/read", nil)
	bob.expect(http.StatusNoContent, "POST", "/notifications/read-all", nil)
	var unread struct {
		UnreadNotifications int `json:"unreadNotifications"`
	}
	mustDecode(t, alice.expect(http.StatusOK, "GET", "/me", nil), &unread)
	if unread.UnreadNotifications != 1 {
		t.Fatalf("read-all touched another student: %d unread", unread.UnreadNotifications)
	}

	var page struct {
		Items []struct {
			ID      string     `json:"id"`
			Message string     `json:"message"`
			Link    string     `json:"link"`
			ReadAt  *time.Time `json:"readAt"`
		} `json:"items"`
		Total       int `json:"total"`
		UnreadCount int `json:"unreadCount"`
	}
	mustDecode(t, alice.expect(http.StatusOK, "GET", "/notifications?unread=true", nil), &page)
	if page.Total != 1 || page.UnreadCount != 1 || page.Items[0].ID != aliceNote ||
		page.Items[0].Message != "Bob left Team." || page.Items[0].Link != "/projects/"+projectID {
		t.Fatalf("alice: %+v", page)
	}
	alice.expect(http.StatusNoContent, "POST", "/notifications/"+aliceNote+"/read", nil)
	mustDecode(t, alice.expect(http.StatusOK, "GET", "/notifications?unread=true", nil), &page)
	if page.Total != 0 || page.UnreadCount != 0 || len(page.Items) != 0 {
		t.Fatalf("after reading: %+v", page)
	}
	mustDecode(t, bob.expect(http.StatusOK, "GET", "/notifications", nil), &page)
	if page.Items[0].ID != bobNote || page.Items[0].Link != "/projects/"+projectID+"/rate" || page.Items[0].ReadAt == nil {
		t.Fatalf("bob: %+v", page)
	}
}

func TestViewerActionsListOnlyMyOpenTeams(t *testing.T) {
	api := newTestAPI(t, true)
	lead := api.signIn(aliceEmail)
	lead.completeProfile("Lead", "Go")
	student := api.signIn(bobEmail)
	student.completeProfile("Student", "Go")

	open, openRole := api.insertTeam(lead, "Open", time.Time{})
	joined, joinedRole := api.insertTeam(lead, "Joined", time.Time{})
	api.addMember(joined, joinedRole, student)
	closed, _ := api.insertTeam(lead, "Closed", time.Time{})
	api.exec(`UPDATE projects SET recruitment_status = 'closed' WHERE id = $1`, closed)
	api.insertTeam(student, "Theirs", time.Time{})

	var profile struct {
		ViewerActions struct {
			InvitableProjects []struct {
				ID    string `json:"id"`
				Roles []struct {
					ID        string `json:"id"`
					Available int    `json:"available"`
				} `json:"roles"`
			} `json:"invitableProjects"`
		} `json:"viewerActions"`
	}
	mustDecode(t, lead.expect(http.StatusOK, "GET", "/profiles/"+student.userID, nil), &profile)
	projects := profile.ViewerActions.InvitableProjects
	if len(projects) != 1 || projects[0].ID != open || len(projects[0].Roles) != 1 || projects[0].Roles[0].ID != openRole || projects[0].Roles[0].Available != 9 {
		t.Fatalf("invitable: %+v", projects)
	}
}
