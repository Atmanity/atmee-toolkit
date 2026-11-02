package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testVariant = Variant{Name: "atmee-x", BaseURL: "https://x.example", BaseURLEnv: "ATMEE_X_BASE_URL",
	HomeFolder: ".atmee-x", HomeEnv: "ATMEE_X_HOME", APIKeyEnv: "ATMEE_X_API_KEY"}

func runAs(t *testing.T, stdin string, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := ExecuteAs(testVariant, args, &out, &errOut, strings.NewReader(stdin))
	return out.String(), errOut.String(), code
}

func TestVariantHelpUsesItsNames(t *testing.T) {
	out, _, code := runAs(t, "", "--help")
	if code != 0 {
		t.Fatal(code)
	}
	for _, want := range []string{"ATMEE_X_BASE_URL (default https://x.example", "`atmee-x login`", "  atmee-x balance", "atmee-x [command]", "ATMEE_X_API_KEY"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "atmee login") || strings.Contains(out, "ATMEE_API_KEY") || strings.Contains(out, "ATMEE_BASE_URL") || strings.Contains(out, "api.atmanity.us") {
		t.Fatalf("help keeps the default names:\n%s", out)
	}
	out, _, _ = runAs(t, "", "--version")
	if !strings.HasPrefix(out, "atmee-x version ") {
		t.Fatal(out)
	}
}

func TestVariantRenamesEveryTopLevelCommandInHints(t *testing.T) {
	a := newApp(testVariant, &bytes.Buffer{}, &bytes.Buffer{}, strings.NewReader(""))
	root := a.root()
	for _, c := range root.Commands() {
		if got, want := a.rename("run `atmee "+c.Name()+"`"), "run `atmee-x "+c.Name()+"`"; got != want {
			t.Fatalf("%q, want %q", got, want)
		}
	}
	for _, same := range []string{"sk_atmee_x", "the atmee server", "`atmee.tool`", "written as ATMEE_API_KEY=..."} {
		if got := a.rename(same); got != same {
			t.Fatalf("%q became %q", same, got)
		}
	}
}

func TestVariantLeavesValuesAlone(t *testing.T) {
	var out bytes.Buffer
	a := newApp(testVariant, &out, &out, strings.NewReader(""))
	a.root()
	a.printf("%s `atmee login`\n", "ATMEE_API_KEY and atmee avatars")
	a.noteMissing("id", []string{"image"})
	got := out.String()
	for _, want := range []string{"ATMEE_API_KEY and atmee avatars `atmee-x login`\n", "  atmee-x avatars replace-appearance id PORTRAIT\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("lacks %q:\n%s", want, got)
		}
	}
}

func TestVariantKeepsToItsLogin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ATMEE_X_HOME", "")
	t.Setenv("ATMEE_API_KEY", "sk_atmee_other")
	t.Setenv("ATMEE_X_API_KEY", "")
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits++ }))
	defer srv.Close()
	t.Setenv("ATMEE_X_BASE_URL", srv.URL)

	_, errOut, code := runAs(t, "", "balance")
	if code == 0 || !strings.Contains(errOut, "`atmee-x login`") || !strings.Contains(errOut, "ATMEE_X_API_KEY") {
		t.Fatalf("%d %s", code, errOut)
	}
	if hits != 0 {
		t.Fatal("the default variant's key reached the server")
	}

	if _, errOut, code := runAs(t, "sk_atmee_saved\n", "login", "--api-key"); code != 0 {
		t.Fatal(errOut)
	}
	if _, err := os.Stat(filepath.Join(home, ".atmee-x", "credentials.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".atmee")); !os.IsNotExist(err) {
		t.Fatalf("touched the default folder: %v", err)
	}
}

func TestVariantHonoursItsKeyAndOverrides(t *testing.T) {
	t.Setenv("ATMEE_X_HOME", t.TempDir())
	t.Setenv("ATMEE_X_API_KEY", "sk_atmee_mine")
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Api-Key")
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	t.Setenv("ATMEE_X_BASE_URL", srv.URL)
	runAs(t, "", "balance")
	if got != "sk_atmee_mine" {
		t.Fatalf("sent %q", got)
	}
}

func TestDefaultVariantTextIsUntouched(t *testing.T) {
	isolate(t)
	out, _, _ := run(t, "", "--help")
	for _, want := range []string{"default https://api.atmanity.us", "`atmee login`", "ATMEE_API_KEY", "atmee [command]"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help lacks %q:\n%s", want, out)
		}
	}
}

// atmee has the shared commands only: a command of one build alone comes
// from that build's Variant.Extra.
func TestAtmeeHasNoExtraCommands(t *testing.T) {
	if len(defaultVariant.Extra) != 0 {
		t.Fatalf("atmee adds %d commands of its own", len(defaultVariant.Extra))
	}
	root := newApp(defaultVariant, &bytes.Buffer{}, &bytes.Buffer{}, strings.NewReader("")).root()
	if c, _, err := root.Find([]string{"text"}); err == nil && c != root {
		t.Fatalf("atmee has %q", c.CommandPath())
	}
}
