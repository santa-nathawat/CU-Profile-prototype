package httpapi

import (
	"io"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const apiPrefix = "/api/v1"

type Options struct {
	DB           *pgxpool.Pool // nil means every route except health answers 503
	DemoSignIn   bool
	CookieSecure bool
}

type Server struct {
	db           *pgxpool.Pool
	demoSignIn   bool
	cookieSecure bool
	mux          *http.ServeMux
}

// NewHandler is the API with no database: health works, other routes return 503.
func NewHandler() http.Handler {
	return New(Options{})
}

func New(opts Options) *Server {
	s := &Server{db: opts.DB, demoSignIn: opts.DemoSignIn, cookieSecure: opts.CookieSecure, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET "+apiPrefix+"/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"status":"ok"}`)
	})
	// Each domain keeps its routes in its own file. A new domain adds a file and a line here.
	s.registerAccounts()
	s.registerProfiles()
	s.registerProfileActivity()
	s.registerCV()
	s.registerNotifications()
	s.registerProjects()
	s.registerRequests()
	s.registerTeamLifecycle()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// handle registers a route under /api/v1, for example "GET /me". Mutating routes
// must send application/json, which a cross-site HTML form can't do.
func (s *Server) handle(pattern string, h http.HandlerFunc) {
	s.register(pattern, "application/json", h)
}

// handleUpload is handle for routes that take multipart/form-data.
func (s *Server) handleUpload(pattern string, h http.HandlerFunc) {
	s.register(pattern, "multipart/form-data", h)
}

func (s *Server) register(pattern, mediaType string, h http.HandlerFunc) {
	method, path, _ := strings.Cut(pattern, " ")
	mutating := method != http.MethodGet
	s.mux.HandleFunc(method+" "+apiPrefix+path, func(w http.ResponseWriter, r *http.Request) {
		if s.db == nil {
			writeError(w, http.StatusServiceUnavailable, "database_unavailable", "The service is not available right now.")
			return
		}
		if mutating && !hasMediaType(r, mediaType) {
			writeError(w, http.StatusBadRequest, "unsupported_content_type", "This request must be sent as "+mediaType+".")
			return
		}
		h(w, r)
	})
}
