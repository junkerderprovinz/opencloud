// Command brandingd serves the API of the branding admin app on loopback,
// behind the OpenCloud proxy route /brandingsvc/.
package main

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/junkerderprovinz/opencloud/branding/server/internal/api"
	"github.com/junkerderprovinz/opencloud/branding/server/internal/auth"
	"github.com/junkerderprovinz/opencloud/branding/server/internal/theme"
)

func main() {
	listen := env("BRANDING_LISTEN", "127.0.0.1:9299")
	dataDir := env("BRANDING_DATA_DIR", "/var/lib/opencloud")
	basePath := env("BRANDING_BASE_THEME", "/usr/local/share/opencloud-branding/base-theme.json")
	openCloud := env("BRANDING_OPENCLOUD_URL", "https://127.0.0.1:9200")

	if !isLoopback(listen) {
		log.Fatalf("brandingd: refusing to listen on %s, only loopback is allowed", listen)
	}
	target, err := url.Parse(openCloud)
	if err != nil || !isLoopback(target.Host) {
		log.Fatalf("brandingd: BRANDING_OPENCLOUD_URL must point at loopback, got %q", openCloud)
	}
	base, err := theme.LoadBase(basePath)
	if err != nil {
		log.Fatalf("brandingd: %v", err)
	}
	store := &theme.Store{
		AssetsDir: filepath.Join(dataDir, "web", "assets", "themes", "_branding"),
		StateFile: filepath.Join(dataDir, "branding", "state.json"),
		Base:      base,
	}
	if err := store.Regenerate(); err != nil {
		log.Fatalf("brandingd: rebuilding the theme overlay: %v", err)
	}
	handler := (&api.Server{
		Store: store,
		Auth: auth.Checker{
			BaseURL: openCloud,
			Client: &http.Client{
				Timeout: 15 * time.Second,
				// OpenCloud's certificate is self-signed or issued for the public
				// name, never for 127.0.0.1. The target was checked to be loopback.
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // loopback only
				},
				CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
			},
		},
		LoginBackgroundActive: os.Getenv("BRANDING_LOGIN_BACKGROUND_ACTIVE") == "true",
	}).Handler()
	srv := &http.Server{
		Addr:              listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute,
		WriteTimeout:      2 * time.Minute,
	}
	log.Printf("brandingd: listening on %s", listen)
	log.Fatal(srv.ListenAndServe())
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func isLoopback(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
