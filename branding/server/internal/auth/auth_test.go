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

func fakeOpenCloud(t *testing.T, meStatus int, permsStatus int, permsBody string) (Checker, *[]string) {
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
			w.WriteHeader(permsStatus)
			if permsBody != "" {
				_, _ = io.WriteString(w, permsBody)
			}
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
	c, seen := fakeOpenCloud(t, http.StatusOK, http.StatusCreated, `{"permissions":["Settings.ReadWrite.all","Logo.Write.all"]}`)
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
		meStatus    int
		permsStatus int
		permsBody   string
	}{
		{"me unauthorised", http.StatusUnauthorized, http.StatusCreated, `{"permissions":["Logo.Write.all"]}`},
		{"no logo permission", http.StatusOK, http.StatusCreated, `{"permissions":["Settings.ReadWrite.all"]}`},
		{"permissions-list wrong status 200", http.StatusOK, http.StatusOK, `{"permissions":["Logo.Write.all"]}`},
		{"permissions-list empty body (real bad token)", http.StatusOK, http.StatusOK, ""},
		{"invalid json", http.StatusOK, http.StatusCreated, `not json`},
		{"prefix is not enough", http.StatusOK, http.StatusCreated, `{"permissions":["Logo.Write"]}`},
		{"me empty id", http.StatusOK, http.StatusCreated, `{"permissions":["Logo.Write.all"]}`},
	}
	for _, tc := range cases {
		c, seen := fakeOpenCloud(t, tc.meStatus, tc.permsStatus, tc.permsBody)
		if tc.name == "me empty id" {
			// Inject empty ID for this specific test
			*seen = (*seen)[0:0]
			c2 := Checker{BaseURL: c.BaseURL, Client: c.Client}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/graph/v1.0/me":
					w.WriteHeader(http.StatusOK)
					_, _ = io.WriteString(w, `{"id":"","displayName":"Admin"}`)
				case "/api/v0/settings/permissions-list":
					w.WriteHeader(http.StatusCreated)
					_, _ = io.WriteString(w, `{"permissions":["Logo.Write.all"]}`)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer srv.Close()
			c2.BaseURL = srv.URL
			c2.Client = srv.Client()
			if err := c2.Check(context.Background(), "Bearer tok"); !errors.Is(err, ErrForbidden) {
				t.Errorf("%s: want ErrForbidden, got %v", tc.name, err)
			}
			continue
		}
		if err := c.Check(context.Background(), "Bearer tok"); !errors.Is(err, ErrForbidden) {
			t.Errorf("%s: want ErrForbidden, got %v", tc.name, err)
		}
	}
}

func TestMeIdMismatch(t *testing.T) {
	// Account ID mismatch causes fake to return 404
	t.Helper()
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path+" "+r.Header.Get("Authorization"))
		switch r.URL.Path {
		case "/graph/v1.0/me":
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"id":"different-user","displayName":"Admin"}`)
		case "/api/v0/settings/permissions-list":
			var body struct {
				AccountUUID string `json:"account_uuid"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.AccountUUID != "user-1" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"permissions":["Logo.Write.all"]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := Checker{BaseURL: srv.URL, Client: srv.Client()}
	if err := c.Check(context.Background(), "Bearer tok"); !errors.Is(err, ErrForbidden) {
		t.Errorf("me id mismatch: want ErrForbidden, got %v", err)
	}
}
