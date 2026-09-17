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

func fakeOpenCloud(t *testing.T, meStatus int, permsBody string) (Checker, *[]string) {
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/graph/v1.0/me":
			w.WriteHeader(meStatus)
			_, _ = io.WriteString(w, `{"id":"user-1","displayName":"Admin"}`)
		case "/api/v0/settings/permissions-list":
			var body struct {
				AccountUUID string `json:"account_uuid"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.AccountUUID != "user-1" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
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
	c, seen := fakeOpenCloud(t, http.StatusOK, `{"permissions":["Settings.ReadWrite.all","Logo.Write.all"]}`)
	if err := c.Check(context.Background(), "Bearer tok"); err != nil {
		t.Fatalf("admin refused: %v", err)
	}
	if len(*seen) != 2 || !strings.HasSuffix((*seen)[0], "Bearer tok") || !strings.HasPrefix((*seen)[1], "POST /api/v0/settings/permissions-list") {
		t.Errorf("unexpected calls: %v", *seen)
	}
}

func TestRefusals(t *testing.T) {
	cases := []struct {
		name      string
		meStatus  int
		permsBody string
	}{
		{"me unauthorised", http.StatusUnauthorized, `{"permissions":["Logo.Write.all"]}`},
		{"no logo permission", http.StatusOK, `{"permissions":["Settings.ReadWrite.all"]}`},
		{"empty answer on a bad token", http.StatusOK, `{}`},
		{"invalid json", http.StatusOK, `not json`},
		{"prefix is not enough", http.StatusOK, `{"permissions":["Logo.Write"]}`},
	}
	for _, tc := range cases {
		c, _ := fakeOpenCloud(t, tc.meStatus, tc.permsBody)
		if err := c.Check(context.Background(), "Bearer tok"); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s: want ErrForbidden, got %v", tc.name, err)
		}
	}
}
