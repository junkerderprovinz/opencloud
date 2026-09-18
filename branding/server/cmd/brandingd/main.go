// Command brandingd serves the API of the branding admin app on loopback,
// behind the OpenCloud proxy route /brandingsvc/.
package main

import (
	"crypto/tls"
	"flag"
	"fmt"
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
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("brandingd: %v", err)
	}
}

// run serves the API until it fails. With -regenerate it only rewrites the
// theme overlay, which keeps the saved branding current while the app is off.
func run(args []string) error {
	flags := flag.NewFlagSet("brandingd", flag.ContinueOnError)
	regenerate := flags.Bool("regenerate", false, "rewrite the theme overlay from the saved state and exit")
	if err := flags.Parse(args); err != nil {
		return err
	}
	listen := env("BRANDING_LISTEN", "127.0.0.1:9299")
	dataDir := env("BRANDING_DATA_DIR", "/var/lib/opencloud")
	basePath := env("BRANDING_BASE_THEME", "/usr/local/share/opencloud-branding/base-theme.json")
	openCloud := env("BRANDING_OPENCLOUD_URL", "https://127.0.0.1:9200")

	if !isLoopback(listen) {
		return fmt.Errorf("refusing to listen on %s, only loopback is allowed", listen)
	}
	target, err := url.Parse(openCloud)
	if err != nil || !isLoopback(target.Host) {
		return fmt.Errorf("BRANDING_OPENCLOUD_URL must point at loopback, got %q", openCloud)
	}
	base, err := theme.LoadBase(basePath)
	if err != nil {
		return err
	}
	store := &theme.Store{
		AssetsDir: filepath.Join(dataDir, "web", "assets", "themes", "_branding"),
		StateFile: filepath.Join(dataDir, "branding", "state.json"),
		Base:      base,
	}
	if err := store.Regenerate(); err != nil {
		return fmt.Errorf("rebuilding the theme overlay: %w", err)
	}
	if *regenerate {
		return nil
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
	return srv.ListenAndServe()
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
