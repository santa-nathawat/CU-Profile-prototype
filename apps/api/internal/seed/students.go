package seed

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/thinc-org/CU-Profile/apps/api/internal/demo"
)

type slot struct {
	Day   int
	Start string
}

type cvSeed struct {
	FileName string
	Shared   bool
}

type student struct {
	Email        string
	DisplayName  string
	Faculty      string
	Year         int
	Headline     string
	WorkingStyle string
	Skills       []string
	Availability []slot
	CV           *cvSeed
}

// weekly makes half-hour slots from startHour up to endHour on each day (1 is Monday).
func weekly(days []int, startHour, endHour int) []slot {
	slots := []slot{}
	for _, day := range days {
		for hour := startHour; hour < endHour; hour++ {
			slots = append(slots, slot{day, fmt.Sprintf("%02d:00", hour)}, slot{day, fmt.Sprintf("%02d:30", hour)})
		}
	}
	return slots
}

func students() []student {
	return []student{
		{
			Email: "6730123421@student.chula.ac.th", DisplayName: "Mint Thanaporn", Faculty: "Faculty of Engineering", Year: 3,
			Headline:     "Full-stack developer who likes shipping small things fast",
			WorkingStyle: "I work best in short sprints with a clear goal. I reply quickly on Discord and prefer one weekly call.",
			Skills:       []string{"Go", "React", "TypeScript", "SQL", "Backend development", "Pitching"},
			Availability: weekly([]int{2, 4}, 19, 22),
			CV:           &cvSeed{"mint-thanaporn-cv.pdf", true},
		},
		{
			Email: "6830456726@student.chula.ac.th", DisplayName: "Ploy Siriwan", Faculty: "Faculty of Commerce and Accountancy", Year: 2,
			Headline:     "Marketing student looking for a startup to grow",
			WorkingStyle: "Organised and deadline driven. I keep a shared task list and like to plan the week on Sunday.",
			Skills:       []string{"Digital marketing", "Financial modeling", "Business strategy", "Excel", "Pitching"},
			Availability: weekly([]int{1, 3, 5}, 18, 20),
			CV:           &cvSeed{"ploy-siriwan-cv.pdf", false},
		},
		{
			Email: "6630789035@student.chula.ac.th", DisplayName: "Tonkla Pakorn", Faculty: "Faculty of Fine and Applied Arts", Year: 4,
			Headline:     "Product designer with a soft spot for type and illustration",
			WorkingStyle: "I sketch first and share early. Happy to work late, and I like honest design feedback.",
			Skills:       []string{"UI design", "Figma", "Illustration", "Branding", "Graphic design"},
			Availability: weekly([]int{6, 7}, 13, 17),
			CV:           &cvSeed{"tonkla-portfolio.pdf", true},
		},
		{
			Email: "6730234528@student.chula.ac.th", DisplayName: "Fah Napat", Faculty: "Faculty of Communication Arts", Year: 3,
			Headline:     "Storyteller for short video and social",
			WorkingStyle: "I like clear briefs and quick feedback. I work in bursts around filming days.",
			Skills:       []string{"Video editing", "Content writing", "Social media", "Photography", "Public relations"},
			Availability: weekly([]int{2, 3}, 16, 18),
		},
		{
			Email: "6730567823@student.chula.ac.th", DisplayName: "Beam Kittipong", Faculty: "Faculty of Science", Year: 3,
			Headline:     "Maths and data person who enjoys messy datasets",
			WorkingStyle: "Quiet and focused. I prefer written updates, and I check the numbers twice before sharing.",
			Skills:       []string{"Python", "Data analysis", "Statistics", "Data visualization", "Machine learning", "Lab research"},
			Availability: weekly([]int{1, 2, 3, 4}, 20, 22),
		},
		{
			Email: "6830345629@student.chula.ac.th", DisplayName: "Kong Chayanon", Faculty: "Faculty of Economics", Year: 2,
			Headline:     "Curious about markets and how small businesses make money",
			WorkingStyle: "I like talking problems through in person, then writing down what we decided.",
			Skills:       []string{"Market research", "Survey design", "Excel", "User interviews", "Sales"},
			Availability: weekly([]int{5}, 13, 18),
		},
		{
			Email: "6530678925@student.chula.ac.th", DisplayName: "Praew Rinrada", Faculty: "Faculty of Architecture", Year: 5,
			Headline:     "Spatial designer interested in community projects",
			WorkingStyle: "I care about the whole picture and plan around studio deadlines, so I like knowing the schedule early.",
			Skills:       []string{"Rhino 3D", "UX research", "Project management", "Community building", "Graphic design"},
			Availability: weekly([]int{6}, 9, 12),
		},
		{
			Email: "6630890130@student.chula.ac.th", DisplayName: "Nine Worawit", Faculty: "Faculty of Medicine", Year: 4,
			Headline:     "Medical student who wants to build better health tools",
			WorkingStyle: "My timetable changes a lot, so I work best async and keep my tasks small and well defined.",
			Skills:       []string{"Clinical research", "Academic writing", "Statistics", "Pitching"},
			Availability: weekly([]int{7}, 10, 12),
			CV:           &cvSeed{"nine-worawit-cv.pdf", true},
		},
	}
}

func seedStudents(ctx context.Context, tx pgx.Tx) error {
	all := students()
	for i, s := range all {
		if err := upsertStudent(ctx, tx, s, len(all)-i); err != nil {
			return fmt.Errorf("seed %s: %w", s.Email, err)
		}
	}
	_, err := upsertUser(ctx, tx, demo.NewStudentEmail)
	return err
}

func upsertUser(ctx context.Context, tx pgx.Tx, email string) (string, error) {
	var id string
	// Clearing closed_at brings back a demo account someone closed while trying that flow.
	err := tx.QueryRow(ctx, `
		INSERT INTO users (email, oauth_subject) VALUES ($1, 'demo:' || $1)
		ON CONFLICT (oauth_subject) DO UPDATE SET closed_at = NULL, updated_at = now()
		RETURNING id`, email).Scan(&id)
	return id, err
}

// upsertStudent writes one complete profile. daysAgo spreads created_at so the
// directory's newest-first order is stable, and only applies on first insert.
func upsertStudent(ctx context.Context, tx pgx.Tx, s student, daysAgo int) error {
	id, err := upsertUser(ctx, tx, s.Email)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO profiles (user_id, display_name, faculty_id, year, headline, working_style, created_at)
		SELECT $1, $2, f.id, $4, $5, $6, now() - $7 * interval '1 day' FROM faculties f WHERE f.name = $3
		ON CONFLICT (user_id) DO UPDATE SET
			display_name = EXCLUDED.display_name, faculty_id = EXCLUDED.faculty_id, year = EXCLUDED.year,
			headline = EXCLUDED.headline, working_style = EXCLUDED.working_style, updated_at = now()`,
		id, s.DisplayName, s.Faculty, s.Year, s.Headline, s.WorkingStyle, daysAgo)
	if err != nil {
		return err
	}
	if err := replaceStudentSkills(ctx, tx, id, s.Skills); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM availability_slots WHERE user_id = $1`, id); err != nil {
		return err
	}
	for _, slot := range s.Availability {
		_, err := tx.Exec(ctx, `INSERT INTO availability_slots (user_id, day_of_week, start_time) VALUES ($1, $2, $3::time)`, id, slot.Day, slot.Start)
		if err != nil {
			return err
		}
	}
	if s.CV == nil {
		return nil
	}
	return upsertCV(ctx, tx, id, s)
}

func replaceStudentSkills(ctx context.Context, tx pgx.Tx, userID string, names []string) error {
	if _, err := tx.Exec(ctx, `DELETE FROM user_skills WHERE user_id = $1`, userID); err != nil {
		return err
	}
	for i, name := range names {
		if _, err := tx.Exec(ctx, `INSERT INTO skills (name) VALUES ($1) ON CONFLICT (normalized_name) DO NOTHING`, name); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			INSERT INTO user_skills (user_id, skill_id, created_at)
			SELECT $1, id, now() + $3 * interval '1 millisecond' FROM skills WHERE normalized_name = lower(trim($2))`,
			userID, name, i)
		if err != nil {
			return err
		}
	}
	return nil
}

func upsertCV(ctx context.Context, tx pgx.Tx, userID string, s student) error {
	content := samplePDF(s.DisplayName)
	_, err := tx.Exec(ctx, `
		INSERT INTO cv_files (user_id, content) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET content = EXCLUDED.content`, userID, content)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE profiles SET cv_storage_key = 'db:' || user_id, cv_file_name = $2, cv_size_bytes = $3,
			cv_uploaded_at = now(), cv_shared = $4
		WHERE user_id = $1`, userID, s.CV.FileName, len(content), s.CV.Shared)
	return err
}

// samplePDF builds a one-page PDF that opens in a real viewer, with a correct cross-reference table.
func samplePDF(name string) []byte {
	stream := fmt.Sprintf("BT /F1 24 Tf 72 720 Td (%s) Tj 0 -32 Td /F1 14 Tf (Sample CV for the demo) Tj ET", name)
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Contents 4 0 R /Resources << /Font << /F1 5 0 R >> >> >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	var xref strings.Builder
	fmt.Fprintf(&xref, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for i, object := range objects {
		fmt.Fprintf(&xref, "%010d 00000 n \n", out.Len())
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	start := out.Len()
	out.WriteString(xref.String())
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, start)
	return out.Bytes()
}
