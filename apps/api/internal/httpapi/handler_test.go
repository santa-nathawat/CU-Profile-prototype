package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutes(t *testing.T) {
	server := httptest.NewServer(NewHandler())
	defer server.Close()
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "/api/v1/health", http.StatusOK},
		{"HEAD", "/api/v1/health", http.StatusOK},
		{"POST", "/api/v1/health", http.StatusMethodNotAllowed},
		{"GET", "/missing", http.StatusNotFound},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			request, _ := http.NewRequest(tc.method, server.URL+tc.path, nil)
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status {
				t.Fatalf("status: got %d, want %d", response.StatusCode, tc.status)
			}
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if tc.method == "GET" && tc.status == http.StatusOK {
				if response.Header.Get("Content-Type") != "application/json" || string(body) != `{"status":"ok"}` {
					t.Fatalf("unexpected health response: %s", body)
				}
			}
			if tc.method == "HEAD" && len(body) != 0 {
				t.Fatal("HEAD returned a body")
			}
		})
	}
}

func TestRoutesAnswer503WithoutDatabase(t *testing.T) {
	server := httptest.NewServer(NewHandler())
	defer server.Close()
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/v1/me"},
		{"GET", "/api/v1/profiles"},
		{"GET", "/api/v1/notifications"},
		{"POST", "/api/v1/auth/sign-out"},
		{"POST", "/api/v1/me/cv"},
	} {
		request, _ := http.NewRequest(tc.method, server.URL+tc.path, nil)
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		if response.StatusCode != http.StatusServiceUnavailable || !strings.Contains(string(body), `"code":"database_unavailable"`) {
			t.Errorf("%s %s: %d %s", tc.method, tc.path, response.StatusCode, body)
		}
	}
	// Demo sign-in is off here, so the route doesn't exist at all.
	response, err := http.Post(server.URL+"/api/v1/auth/demo-sign-in", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Errorf("demo sign-in: %d", response.StatusCode)
	}
}
