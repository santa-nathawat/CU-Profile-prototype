// Package demo names the seeded demo accounts. The seed creates them and the
// sign-in page lists them, so both read this one list.
package demo

type Account struct {
	Email string
	Note  string
}

// NewStudentEmail has no profile, for trying the first-run flow.
const NewStudentEmail = "6830000121@student.chula.ac.th"

var Accounts = []Account{
	{"6730123421@student.chula.ac.th", "Engineering, year 3. Project lead. Gave two ratings and received one"},
	{"6830456726@student.chula.ac.th", "Commerce, year 2. Marketer. Gave and received a rating"},
	{"6630789035@student.chula.ac.th", "Fine Arts, year 4. Designer. Received a rating but gave none"},
	{"6730234528@student.chula.ac.th", "Communication Arts, year 3. Content and photography"},
	{"6730567823@student.chula.ac.th", "Science, year 3. Developer. No ratings given or received"},
	{"6830345629@student.chula.ac.th", "Economics, year 2. Market research"},
	{"6530678925@student.chula.ac.th", "Architecture, year 5. Spatial design"},
	{"6630890130@student.chula.ac.th", "Medicine, year 4. Health research"},
	{NewStudentEmail, "New student with no profile yet"},
}

func Emails() []string {
	emails := make([]string, len(Accounts))
	for i, account := range Accounts {
		emails[i] = account.Email
	}
	return emails
}
