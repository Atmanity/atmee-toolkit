package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Atmanity/atmee-toolkit/cli/internal/api"
)

// On an account with several people the API answers earnings null to anyone
// but the owner: the profile says so instead of printing zeros or crashing.
func TestPrintMonetizationWithoutEarnings(t *testing.T) {
	var out bytes.Buffer
	a := &app{out: &out, err: &out, rename: func(s string) string { return s }}
	a.printMonetization(&api.Monetization{})
	if !strings.Contains(out.String(), "only the account's owner can see them") {
		t.Fatalf("output: %q", out.String())
	}
}
