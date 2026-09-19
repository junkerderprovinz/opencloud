package auth

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeOpenCloud answers /me with meID and lists permissions only for that
// account.
func fakeOpenCloud(t *testing.T, meID string, meStatus, permsStatus int, permsBody string) (Checker, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/graph/v1.0/me":
			w.WriteHeader(meStatus)
			_ = json.NewEncoder(w).Encode(map[string]string{"id": meID, "displayName": "Admin"})
		case "/api/v0/settings/permissions-list":
			var body struct {
				AccountUUID string `json:"account_uuid"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.AccountUUID != meID {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(permsStatus)
			_, _ = io.WriteString(w, permsBody)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return Checker{BaseURL: srv.URL, Client: srv.Client()}, &seen
}

func TestNoCredentials(t *testing.T) {
	if err := (Checker{}).Check(context.Background(), "  "); !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("want ErrNoCredentials, got %v", err)
	}
}

func TestAdminAllowed(t *testing.T) {
	c, seen := fakeOpenCloud(t, "admin-7", http.StatusOK, http.StatusCreated, `{"permissions":["Settings.ReadWrite.all","Logo.Write.all"]}`)
	if err := c.Check(context.Background(), "Bearer tok"); err != nil {
		t.Fatalf("admin refused: %v", err)
	}
	if len(*seen) != 2 || !strings.HasSuffix((*seen)[0], "Bearer tok") || !strings.HasPrefix((*seen)[1], "POST /api/v0/settings/permissions-list") {
		t.Errorf("unexpected calls: %v", *seen)
	}
}

func TestRefusals(t *testing.T) {
	cases := []struct {
		name        string
		meID        string
		meStatus    int
		permsStatus int
		permsBody   string
	}{
		{"me unauthorised", "user-1", http.StatusUnauthorized, http.StatusCreated, `{"permissions":["Logo.Write.all"]}`},
		{"me empty id", "", http.StatusOK, http.StatusCreated, `{"permissions":["Logo.Write.all"]}`},
		{"no logo permission", "user-1", http.StatusOK, http.StatusCreated, `{"permissions":["Settings.ReadWrite.all"]}`},
		{"permissions-list wrong status 200", "user-1", http.StatusOK, http.StatusOK, `{"permissions":["Logo.Write.all"]}`},
		{"permissions-list empty body at 200 (real bad token)", "user-1", http.StatusOK, http.StatusOK, ""},
		{"permissions-list empty body at 201", "user-1", http.StatusOK, http.StatusCreated, ""},
		{"invalid json", "user-1", http.StatusOK, http.StatusCreated, `not json`},
		{"prefix is not enough", "user-1", http.StatusOK, http.StatusCreated, `{"permissions":["Logo.Write"]}`},
	}
	for _, tc := range cases {
		c, _ := fakeOpenCloud(t, tc.meID, tc.meStatus, tc.permsStatus, tc.permsBody)
		if err := c.Check(context.Background(), "Bearer tok"); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s: want ErrForbidden, got %v", tc.name, err)
		}
	}
}
