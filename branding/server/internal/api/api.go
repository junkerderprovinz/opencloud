// Package api is the HTTP surface of brandingd, reached through the OpenCloud
// proxy route /brandingsvc/.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/junkerderprovinz/opencloud/branding/server/internal/auth"
	"github.com/junkerderprovinz/opencloud/branding/server/internal/imagefmt"
	"github.com/junkerderprovinz/opencloud/branding/server/internal/svgclean"
	"github.com/junkerderprovinz/opencloud/branding/server/internal/theme"
)

// Authorizer decides whether a request may read or change the branding.
type Authorizer interface {
	Check(ctx context.Context, authorization string) error
}

// Server holds the dependencies of the handlers.
type Server struct {
	Store *theme.Store
	Auth  Authorizer
	// LoginBackgroundActive says whether the IDP was started with a
	// background URL. Only a container restart changes that.
	LoginBackgroundActive bool
}

// view is the state as the web app sees it; image fields are URLs.
type view struct {
	Name                  string `json:"name"`
	Slogan                string `json:"slogan"`
	Logo                  string `json:"logo"`
	LogoDark              string `json:"logoDark"`
	Favicon               string `json:"favicon"`
	Background            string `json:"background"`
	LoginBackgroundActive bool   `json:"loginBackgroundActive"`
}

// Handler returns the routes. Method patterns make every other method,
// OPTIONS included, answer 405.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /brandingsvc/health", s.health)
	mux.HandleFunc("GET /brandingsvc/login-background", s.loginBackground)
	mux.HandleFunc("GET /brandingsvc/api/state", s.guard(s.getState))
	mux.HandleFunc("PUT /brandingsvc/api/text", s.guard(s.putText))
	mux.HandleFunc("PUT /brandingsvc/api/image/{kind}", s.guard(s.putImage))
	mux.HandleFunc("DELETE /brandingsvc/api/image/{kind}", s.guard(s.deleteImage))
	return mux
}

// guard requires the X-Branding-Request header, which a cross-site form or
// image request cannot set, and then asks OpenCloud about the caller.
func (s *Server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Branding-Request") != "1" {
			http.Error(w, "missing X-Branding-Request header", http.StatusForbidden)
			return
		}
		switch err := s.Auth.Check(r.Context(), r.Header.Get("Authorization")); {
		case err == nil:
			next(w, r)
		case errors.Is(err, auth.ErrNoCredentials):
			http.Error(w, "authentication required", http.StatusUnauthorized)
		default:
			// A bare ErrForbidden is an ordinary refusal. A wrapped one carries
			// an upstream failure, such as a wrong BRANDING_OPENCLOUD_URL, that
			// the admin only sees as the same 403.
			if err != auth.ErrForbidden {
				log.Printf("brandingd: permission check: %v", err)
			}
			http.Error(w, "forbidden", http.StatusForbidden)
		}
	}
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "ok\n")
}

func (s *Server) loginBackground(w http.ResponseWriter, r *http.Request) {
	st, err := s.Store.Load()
	if err != nil {
		log.Printf("brandingd: %v", err)
		http.Error(w, "state unavailable", http.StatusInternalServerError)
		return
	}
	if st.Background == "" {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(filepath.Join(s.Store.AssetsDir, st.Background))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		log.Printf("brandingd: %v", err)
		http.Error(w, "background unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("ETag", `"`+st.Background+`"`)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// An SVG background opened on its own must not run anything, whatever
	// the sanitiser missed. As a CSS background the policy does not apply.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; img-src data:; style-src 'unsafe-inline'; sandbox")
	http.ServeContent(w, r, st.Background, info.ModTime(), f)
}

func (s *Server) getState(w http.ResponseWriter, _ *http.Request) {
	st, err := s.Store.Load()
	if err != nil {
		log.Printf("brandingd: %v", err)
		http.Error(w, "loading failed", http.StatusInternalServerError)
		return
	}
	s.respond(w, st, nil)
}

func (s *Server) putText(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name   string `json:"name"`
		Slogan string `json:"slogan"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	st, err := s.Store.SetText(body.Name, body.Slogan)
	s.respond(w, st, err)
}

func (s *Server) putImage(w http.ResponseWriter, r *http.Request) {
	kind, ok := imagefmt.ParseKind(r.PathValue("kind"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, imagefmt.MaxBytes(kind)))
	var tooLarge *http.MaxBytesError
	switch {
	case errors.As(err, &tooLarge):
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	case err != nil:
		http.Error(w, "upload failed", http.StatusBadRequest)
		return
	}
	ext, ok := imagefmt.Raster(data)
	if !ok {
		if !imagefmt.LooksLikeSVG(data) {
			http.Error(w, "unsupported image format", http.StatusUnsupportedMediaType)
			return
		}
		data, err = svgclean.Sanitize(bytes.NewReader(data))
		switch {
		case errors.Is(err, svgclean.ErrTooComplex):
			http.Error(w, "SVG too complex", http.StatusUnsupportedMediaType)
			return
		case err != nil:
			http.Error(w, "invalid SVG", http.StatusUnsupportedMediaType)
			return
		}
		ext = "svg"
	}
	st, err := s.Store.SetImage(kind, ext, data)
	s.respond(w, st, err)
}

func (s *Server) deleteImage(w http.ResponseWriter, r *http.Request) {
	kind, ok := imagefmt.ParseKind(r.PathValue("kind"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	st, err := s.Store.ClearImage(kind)
	s.respond(w, st, err)
}

func (s *Server) respond(w http.ResponseWriter, st theme.State, err error) {
	switch {
	case errors.Is(err, theme.ErrTooLong):
		http.Error(w, "name or slogan too long", http.StatusBadRequest)
		return
	case err != nil:
		log.Printf("brandingd: %v", err)
		http.Error(w, "saving failed", http.StatusInternalServerError)
		return
	}
	url := func(file string) string {
		if file == "" {
			return ""
		}
		return "/" + theme.AssetPrefix + file
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(view{
		Name:                  st.Name,
		Slogan:                st.Slogan,
		Logo:                  url(st.Logo),
		LogoDark:              url(st.LogoDark),
		Favicon:               url(st.Favicon),
		Background:            url(st.Background),
		LoginBackgroundActive: s.LoginBackgroundActive,
	})
}
