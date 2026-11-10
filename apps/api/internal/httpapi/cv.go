package httpapi

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const (
	maxCVBytes = 5 << 20 // 5 MB
	// The multipart wrapper around the file adds a little on top of the file itself.
	maxCVRequestBytes = maxCVBytes + 1<<16
	maxCVFileName     = 200
	// Readers tolerate bytes after the trailer, so it only has to be near the end.
	pdfTrailerWindow = 1024
)

func (s *Server) registerCV() {
	s.handleUpload("POST /me/cv", s.requireUser(s.uploadCV))
	s.handle("DELETE /me/cv", s.requireUser(s.deleteCV))
	s.handle("PUT /me/cv/sharing", s.requireUser(s.putCVSharing))
	s.handle("GET /users/{id}/cv", s.requireUser(s.getCV))
}

var errCVTooLarge = tooLarge("cv_too_large", "Your CV must be 5 MB or smaller.")
var errNotPDF = badRequest("not_pdf", "Upload a valid PDF that opens without a password.")

// readCVPart streams the "file" field so an oversized upload is refused without being stored anywhere.
func readCVPart(w http.ResponseWriter, r *http.Request) (name string, content []byte, err error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCVRequestBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		return "", nil, badRequest("invalid_upload", "The upload could not be read.")
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return "", nil, badRequest("file_required", "Please choose a PDF to upload.")
		}
		if err != nil {
			return "", nil, uploadReadError(err)
		}
		if part.FormName() != "file" {
			continue
		}
		content, err := io.ReadAll(io.LimitReader(part, maxCVBytes+1))
		if err != nil {
			return "", nil, uploadReadError(err)
		}
		if len(content) > maxCVBytes {
			return "", nil, errCVTooLarge
		}
		return part.FileName(), content, nil
	}
}

func uploadReadError(err error) error {
	var large *http.MaxBytesError
	if errors.As(err, &large) {
		return errCVTooLarge
	}
	return badRequest("invalid_upload", "The upload could not be read.")
}

func cleanCVFileName(name string) string {
	name = strings.TrimSpace(strings.Map(func(c rune) rune {
		if unicode.IsControl(c) {
			return -1
		}
		return c
	}, name))
	if name == "" {
		return "cv.pdf"
	}
	if utf8.RuneCountInString(name) > maxCVFileName {
		name = string([]rune(name)[:maxCVFileName])
	}
	return name
}

func (s *Server) uploadCV(w http.ResponseWriter, r *http.Request, user User) {
	name, content, err := readCVPart(w, r)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if !isPDF(r.Context(), content) {
		writeErr(w, r, errNotPDF)
		return
	}
	name = cleanCVFileName(name)
	ctx := r.Context()
	var uploadedAt time.Time
	err = s.inTx(ctx, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO cv_files (user_id, content) VALUES ($1, $2)
			ON CONFLICT (user_id) DO UPDATE SET content = EXCLUDED.content`, user.ID, content)
		if err != nil {
			return err
		}
		return tx.QueryRow(ctx, `
			INSERT INTO profiles (user_id, cv_storage_key, cv_file_name, cv_size_bytes, cv_uploaded_at)
			VALUES ($1::uuid, 'db:' || $1, $2, $3, now())
			ON CONFLICT (user_id) DO UPDATE SET
				cv_storage_key = EXCLUDED.cv_storage_key, cv_file_name = EXCLUDED.cv_file_name,
				cv_size_bytes = EXCLUDED.cv_size_bytes, cv_uploaded_at = EXCLUDED.cv_uploaded_at, updated_at = now()
			RETURNING cv_uploaded_at`, user.ID, name, len(content)).Scan(&uploadedAt)
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, struct {
		FileName   string    `json:"fileName"`
		SizeBytes  int       `json:"sizeBytes"`
		UploadedAt time.Time `json:"uploadedAt"`
	}{name, len(content), uploadedAt})
}

// deleteCV also turns sharing off, so a later upload starts private rather than
// inheriting a choice made about a different file.
func (s *Server) deleteCV(w http.ResponseWriter, r *http.Request, user User) {
	if err := readEmptyJSON(w, r); err != nil {
		writeErr(w, r, err)
		return
	}
	ctx := r.Context()
	err := s.inTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM cv_files WHERE user_id = $1`, user.ID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `
			UPDATE profiles SET cv_storage_key = NULL, cv_file_name = NULL, cv_size_bytes = NULL,
				cv_uploaded_at = NULL, cv_shared = false, updated_at = now()
			WHERE user_id = $1`, user.ID)
		return err
	})
	if err != nil {
		writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) putCVSharing(w http.ResponseWriter, r *http.Request, user User) {
	var body struct {
		Shared *bool `json:"shared"`
	}
	if err := readJSON(w, r, &body); err != nil {
		writeErr(w, r, err)
		return
	}
	if body.Shared == nil {
		writeErr(w, r, invalidFields(map[string]string{"shared": "Choose whether to share your CV."}))
		return
	}
	_, err := s.db.Exec(r.Context(), `
		INSERT INTO profiles (user_id, cv_shared) VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET cv_shared = EXCLUDED.cv_shared, updated_at = now()`, user.ID, *body.Shared)
	if err != nil {
		writeErr(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"shared": *body.Shared})
}

// getCV checks the sharing flag in the same query that reads the file, on every
// request, and the response is never cached, so turning sharing off blocks old links.
func (s *Server) getCV(w http.ResponseWriter, r *http.Request, user User) {
	errMissing := notFound("cv_not_found", "This student hasn't uploaded a CV.")
	id, ok := pathID(r, "id")
	if !ok {
		writeErr(w, r, errMissing)
		return
	}
	var name string
	var content []byte
	err := s.db.QueryRow(r.Context(), `
		SELECT p.cv_file_name, CASE WHEN p.cv_shared OR p.user_id = $2 THEN f.content END
		FROM profiles p
		JOIN users u ON u.id = p.user_id
		JOIN cv_files f ON f.user_id = p.user_id
		WHERE p.user_id = $1 AND u.closed_at IS NULL`, id, user.ID).Scan(&name, &content)
	if errors.Is(err, pgx.ErrNoRows) {
		writeErr(w, r, errMissing)
		return
	}
	if err != nil {
		writeErr(w, r, err)
		return
	}
	if content == nil {
		writeErr(w, r, forbidden("cv_not_shared", "This student isn't sharing their CV."))
		return
	}
	header := w.Header()
	header.Set("Content-Type", "application/pdf")
	header.Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": name}))
	header.Set("Content-Length", strconv.Itoa(len(content)))
	header.Set("Cache-Control", "private, no-store")
	header.Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}
