// Package auth asks OpenCloud whether a caller may change the branding.
// Nothing the proxy forwards is trusted: the caller's own Authorization
// header goes back to OpenCloud on every request.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var (
	// ErrNoCredentials means the request carried no Authorization header.
	ErrNoCredentials = errors.New("auth: no credentials")
	// ErrForbidden covers every other refusal, including upstream errors.
	ErrForbidden = errors.New("auth: missing Logo.Write.all")
)

const permission = "Logo.Write.all"

// Checker calls OpenCloud through its proxy.
type Checker struct {
	BaseURL string
	Client  *http.Client
}

// Check returns nil only when OpenCloud confirms Logo.Write.all for the
// caller. The permission list comes back empty with status 200 for a token
// the settings service cannot resolve, so an empty list is a refusal too.
func (c Checker) Check(ctx context.Context, authorization string) error {
	if strings.TrimSpace(authorization) == "" {
		return ErrNoCredentials
	}
	var me struct {
		ID string `json:"id"`
	}
	if err := c.call(ctx, http.MethodGet, "/graph/v1.0/me", authorization, nil, http.StatusOK, &me); err != nil {
		return fmt.Errorf("%w: %v", ErrForbidden, err)
	}
	if me.ID == "" {
		return ErrForbidden
	}
	body, err := json.Marshal(map[string]string{"account_uuid": me.ID})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrForbidden, err)
	}
	var perms struct {
		Permissions []string `json:"permissions"`
	}
	if err := c.call(ctx, http.MethodPost, "/api/v0/settings/permissions-list", authorization, body, http.StatusCreated, &perms); err != nil {
		return fmt.Errorf("%w: %v", ErrForbidden, err)
	}
	for _, p := range perms.Permissions {
		if p == permission {
			return nil
		}
	}
	return ErrForbidden
}

func (c Checker) call(ctx context.Context, method, path, authorization string, body []byte, expectedStatus int, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", authorization)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != expectedStatus {
		return fmt.Errorf("%s %s: status %d", method, path, resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out)
}
