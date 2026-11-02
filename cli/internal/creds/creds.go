// Package creds is ~/.atmee/credentials.json: the saved Avatar API key, or
// the session of a user who signed in with the browser. It holds one or the
// other; saving either replaces the file.
package creds

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The environment variables of the default store: the folder that replaces
// ~/.atmee, and an API key.
const (
	DefaultHomeEnv = "ATMEE_HOME"
	DefaultKeyEnv  = "ATMEE_API_KEY"
)

// Store is one credentials file. KeyEnv names the variable whose key wins
// over the file (DefaultKeyEnv when empty).
type Store struct {
	Dir    string
	KeyEnv string
}

// Default is ~/.atmee (ATMEE_HOME overrides).
func Default() Store { return In(".atmee", DefaultHomeEnv, DefaultKeyEnv) }

// In is the store in the folder of that name under the home directory (homeEnv
// overrides), with its API key read from keyEnv. Each variant passes its own
// variables, so one never reads another's saved key.
func In(folder, homeEnv, keyEnv string) Store {
	if d := os.Getenv(homeEnv); d != "" {
		return Store{Dir: d, KeyEnv: keyEnv}
	}
	home, _ := os.UserHomeDir()
	return Store{Dir: filepath.Join(home, folder), KeyEnv: keyEnv}
}

// Session is a browser sign-in: the user's OAuth tokens and what refreshing
// them needs. BaseURL is the Avatar API it was made for; the tokens come from
// that deployment's sign-in and are useless anywhere else.
type Session struct {
	BaseURL       string `json:"base_url"`
	TokenEndpoint string `json:"token_endpoint"`
	ClientID      string `json:"client_id"`
	// Resource is the protected resource the tokens are bound to (RFC 8707);
	// a refresh names it again.
	Resource    string `json:"resource,omitempty"`
	AccessToken string `json:"access_token"`
	// RefreshToken is empty when the server issued none: the sign-in then
	// lasts as long as its access token.
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	Email        string    `json:"email,omitempty"`
	// AccountID is sent as X-Account-Id: which account a user of several
	// acts for. Empty for a user of one account.
	AccountID string `json:"account_id,omitempty"`
}

type file struct {
	APIKey  string   `json:"api_key,omitempty"`
	Session *Session `json:"oauth,omitempty"`
}

func (s Store) path() string { return filepath.Join(s.Dir, "credentials.json") }

func (s Store) read() file {
	var f file
	if b, err := os.ReadFile(s.path()); err == nil {
		_ = json.Unmarshal(b, &f)
	}
	return f
}

// APIKey is the key in KeyEnv, else the saved key ("" when none).
func (s Store) APIKey() string {
	env := s.KeyEnv
	if env == "" {
		env = DefaultKeyEnv
	}
	if k := strings.TrimSpace(os.Getenv(env)); k != "" {
		return k
	}
	return strings.TrimSpace(s.read().APIKey)
}

// Session is the saved browser sign-in, nil when none.
func (s Store) Session() *Session {
	f := s.read()
	if f.Session == nil || f.Session.AccessToken == "" {
		return nil
	}
	return f.Session
}

// SaveAPIKey writes the key, dropping a saved sign-in.
func (s Store) SaveAPIKey(key string) error { return s.write(file{APIKey: key}) }

// SaveSession writes the sign-in, dropping a saved key.
func (s Store) SaveSession(sess Session) error { return s.write(file{Session: &sess}) }

// write creates the file fresh at mode 600 and renames it into place, so its
// secrets are never readable by others, even over an older, wider file.
func (s Store) write(f file) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	b, _ := json.Marshal(f)
	return WriteSecretFile(s.path(), b)
}

// WriteSecretFile writes b to path whole: a fresh file at mode 600 renamed
// into place, so a secret is never readable by others (even over an older,
// wider file) and a reader never sees half of it.
func WriteSecretFile(path string, b []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after the rename
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Remove deletes the saved key or sign-in (and the file).
func (s Store) Remove() error {
	if err := os.Remove(s.path()); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}
