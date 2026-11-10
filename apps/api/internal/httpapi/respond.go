package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"regexp"
	"strings"
)

const maxJSONBody = 1 << 20 // 1 MB

// apiError is an expected failure with a message for the user. Return one from
// a handler or a transaction and writeErr turns it into the error shape.
type apiError struct {
	Status  int
	Code    string
	Message string
	Fields  map[string]string
}

func (e *apiError) Error() string { return e.Code + ": " + e.Message }

func badRequest(code, message string) error {
	return &apiError{Status: http.StatusBadRequest, Code: code, Message: message}
}

func invalidFields(fields map[string]string) error {
	return &apiError{Status: http.StatusBadRequest, Code: "validation_failed", Message: "Please check the highlighted fields.", Fields: fields}
}

func forbidden(code, message string) error {
	return &apiError{Status: http.StatusForbidden, Code: code, Message: message}
}

func notFound(code, message string) error {
	return &apiError{Status: http.StatusNotFound, Code: code, Message: message}
}

func conflict(code, message string) error {
	return &apiError{Status: http.StatusConflict, Code: code, Message: message}
}

func tooLarge(code, message string) error {
	return &apiError{Status: http.StatusRequestEntityTooLarge, Code: code, Message: message}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		log.Printf("encode response: %v", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong. Please try again.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeAPIError(w, &apiError{Status: status, Code: code, Message: message})
}

func writeAPIError(w http.ResponseWriter, e *apiError) {
	type detail struct {
		Code    string            `json:"code"`
		Message string            `json:"message"`
		Fields  map[string]string `json:"fields,omitempty"`
	}
	writeJSON(w, e.Status, map[string]detail{"error": {e.Code, e.Message, e.Fields}})
}

// writeErr answers with an apiError as is. Anything else is logged and hidden behind a 500.
func writeErr(w http.ResponseWriter, r *http.Request, err error) {
	var expected *apiError
	if errors.As(err, &expected) {
		writeAPIError(w, expected)
		return
	}
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	writeError(w, http.StatusInternalServerError, "internal_error", "Something went wrong. Please try again.")
}

// readJSON decodes one JSON value with the contract's limits: 1 MB, no unknown fields.
func readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return jsonError(err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return badRequest("invalid_json", "The request body must be a single JSON value.")
	}
	return nil
}

// readEmptyJSON is for routes whose body is {}. An empty body is fine too.
func readEmptyJSON(w http.ResponseWriter, r *http.Request) error {
	err := readJSON(w, r, &struct{}{})
	var bad *apiError
	if errors.As(err, &bad) && bad.Code == "empty_body" {
		return nil
	}
	return err
}

func jsonError(err error) error {
	var large *http.MaxBytesError
	switch {
	case errors.As(err, &large):
		return tooLarge("body_too_large", "The request body must be 1 MB or smaller.")
	case errors.Is(err, io.EOF):
		return badRequest("empty_body", "The request body is empty.")
	default:
		return badRequest("invalid_json", "The request body is not valid.")
	}
}

func hasMediaType(r *http.Request, want string) bool {
	got, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	return err == nil && strings.EqualFold(got, want)
}

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func validUUID(value string) bool { return uuidPattern.MatchString(value) }

// pathID reads a uuid path value. A malformed id can't match anything, so callers answer 404.
func pathID(r *http.Request, name string) (string, bool) {
	id := r.PathValue(name)
	return strings.ToLower(id), validUUID(id)
}

// likePattern builds a case-insensitive contains pattern, treating the user's % and _ as literal text.
func likePattern(term string) string {
	return "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(term) + "%"
}
