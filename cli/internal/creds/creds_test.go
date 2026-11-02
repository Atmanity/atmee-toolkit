package creds

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveReadRemove(t *testing.T) {
	t.Setenv("ATMEE_API_KEY", "")
	s := Store{Dir: t.TempDir()}
	if s.APIKey() != "" {
		t.Fatal("empty store")
	}
	if err := s.SaveAPIKey("sk_atmee_x"); err != nil {
		t.Fatal(err)
	}
	if s.APIKey() != "sk_atmee_x" {
		t.Fatal(s.APIKey())
	}
	if st, _ := os.Stat(filepath.Join(s.Dir, "credentials.json")); st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode())
	}
	if err := s.Remove(); err != nil || s.APIKey() != "" {
		t.Fatalf("remove: %v %q", err, s.APIKey())
	}
	if err := s.Remove(); err != nil {
		t.Fatal("removing nothing is fine")
	}
}

func TestEnvKeyWins(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	_ = s.SaveAPIKey("sk_atmee_saved")
	t.Setenv("ATMEE_API_KEY", " sk_atmee_env ")
	if s.APIKey() != "sk_atmee_env" {
		t.Fatal(s.APIKey())
	}
}

func TestReadsAnOlderFileAndDropsItsTokensOnSave(t *testing.T) {
	t.Setenv("ATMEE_API_KEY", "")
	s := Store{Dir: t.TempDir()}
	f := filepath.Join(s.Dir, "credentials.json")
	// An older CLI kept gateway login tokens next to the key.
	_ = os.WriteFile(f, []byte(`{"access_token":"a","refresh_token":"r","expires_at":1,"api_key":"sk_atmee_old"}`), 0o644)
	if s.APIKey() != "sk_atmee_old" {
		t.Fatal(s.APIKey())
	}
	if err := s.SaveAPIKey("sk_atmee_new"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(f)
	if string(b) != `{"api_key":"sk_atmee_new"}` {
		t.Fatalf("stale tokens kept: %s", b)
	}
	if st, _ := os.Stat(f); st.Mode().Perm() != 0o600 {
		t.Fatalf("an older, wider file is tightened: %v", st.Mode())
	}
}

func TestKeyAndSignInReplaceEachOther(t *testing.T) {
	t.Setenv("ATMEE_API_KEY", "")
	s := Store{Dir: t.TempDir()}
	if err := s.SaveAPIKey("sk_atmee_x"); err != nil {
		t.Fatal(err)
	}
	sess := Session{BaseURL: "https://api.example", ClientID: "c", AccessToken: "a", RefreshToken: "r", ExpiresAt: time.Now().Add(time.Hour).UTC()}
	if err := s.SaveSession(sess); err != nil {
		t.Fatal(err)
	}
	if s.APIKey() != "" || s.Session() == nil || s.Session().RefreshToken != "r" {
		t.Fatalf("a sign-in replaces the key: %q %+v", s.APIKey(), s.Session())
	}
	if fi, err := os.Stat(filepath.Join(s.Dir, "credentials.json")); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode: %v %v", fi.Mode(), err)
	}
	if err := s.SaveAPIKey("sk_atmee_y"); err != nil || s.Session() != nil || s.APIKey() != "sk_atmee_y" {
		t.Fatalf("a key replaces the sign-in: %v %+v", err, s.Session())
	}
}

func TestInUsesItsFolderAndKeyVariable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ATMEE_HOME", "/default")
	t.Setenv("ATMEE_OTHER_HOME", "")
	t.Setenv("ATMEE_API_KEY", "sk_atmee_default")
	t.Setenv("ATMEE_OTHER_KEY", "")
	s := In(".other", "ATMEE_OTHER_HOME", "ATMEE_OTHER_KEY")
	if s.Dir != filepath.Join(home, ".other") {
		t.Fatal(s.Dir)
	}
	if s.APIKey() != "" {
		t.Fatalf("read another variable: %q", s.APIKey())
	}
	t.Setenv("ATMEE_OTHER_KEY", "sk_atmee_other")
	if s.APIKey() != "sk_atmee_other" {
		t.Fatal(s.APIKey())
	}
	t.Setenv("ATMEE_OTHER_HOME", "/elsewhere")
	if d := In(".other", "ATMEE_OTHER_HOME", "ATMEE_OTHER_KEY").Dir; d != "/elsewhere" {
		t.Fatal(d)
	}
}
