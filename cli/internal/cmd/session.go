package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Atmanity/atmee-toolkit/cli/internal/api"
	"github.com/Atmanity/atmee-toolkit/cli/internal/creds"
	"github.com/Atmanity/atmee-toolkit/cli/internal/oauth"
)

// credential is how the avatar commands authenticate: ATMEE_API_KEY or the
// saved key, else the saved browser sign-in.
func (a *app) credential() (api.Credential, error) {
	if k := a.store.APIKey(); k != "" {
		return api.APIKey(k), nil
	}
	s := a.store.Session()
	if s == nil {
		return nil, errors.New("not signed in: run `atmee login`, or set ATMEE_API_KEY (create one in Atmee Studio -> API keys)")
	}
	if !sameBase(s.BaseURL, a.baseURL()) {
		return nil, fmt.Errorf("signed in to %s, but ATMEE_BASE_URL is %s: run `atmee login` again for this API", s.BaseURL, a.baseURL())
	}
	return &userSession{store: a.store, s: *s, hc: &http.Client{Timeout: 30 * time.Second}}, nil
}

func sameBase(a, b string) bool { return strings.TrimRight(a, "/") == strings.TrimRight(b, "/") }

// accountID is the account a user of several acts for: ATMEE_ACCOUNT_ID, else
// the one chosen at login.
func accountID(s creds.Session) string {
	if v := strings.TrimSpace(os.Getenv("ATMEE_ACCOUNT_ID")); v != "" {
		return v
	}
	return s.AccountID
}

// userSession signs requests as the signed-in user: the access token as
// Authorization: Bearer, refreshed and saved again a minute before it
// expires, and X-Account-Id when an account was chosen.
type userSession struct {
	mu    sync.Mutex
	store creds.Store
	s     creds.Session
	hc    *http.Client
}

func (u *userSession) Authorize(ctx context.Context, r *http.Request) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if time.Until(u.s.ExpiresAt) < time.Minute {
		// Another atmee may have refreshed (and rotated the refresh token)
		// since this one read the file.
		if latest := u.store.Session(); latest != nil && latest.ExpiresAt.After(u.s.ExpiresAt) {
			u.s = *latest
		}
	}
	if time.Until(u.s.ExpiresAt) < time.Minute {
		if u.s.RefreshToken == "" {
			// Nothing to renew with: use the token for as long as it is valid.
			if !time.Now().Before(u.s.ExpiresAt) {
				return errors.New("the sign-in expired (the server issued no refresh token); run `atmee login` again")
			}
			return u.sign(r)
		}
		t, err := oauth.Refresh(ctx, u.hc, u.s.TokenEndpoint, u.s.ClientID, u.s.RefreshToken, u.s.Resource)
		if err != nil {
			return fmt.Errorf("the sign-in expired and could not be renewed (%w); run `atmee login` again", err)
		}
		u.s.AccessToken, u.s.RefreshToken, u.s.ExpiresAt = t.AccessToken, t.RefreshToken, t.ExpiresAt
		if err := u.store.SaveSession(u.s); err != nil {
			return err
		}
	}
	return u.sign(r)
}

func (u *userSession) sign(r *http.Request) error {
	r.Header.Set("Authorization", "Bearer "+u.s.AccessToken)
	if id := accountID(u.s); id != "" {
		r.Header.Set("X-Account-Id", id)
	}
	return nil
}
