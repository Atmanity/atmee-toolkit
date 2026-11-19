package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

const clipAvatar = "11111111-2222-4333-8444-555555555555"

// clipAPI answers every clip with one clip and records the bodies sent.
func clipAPI(t *testing.T) *[]map[string]any {
	t.Helper()
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/generation/clips" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"clip":{"upload":"upload:c","width":480,"height":480,"durationSeconds":4,"expiresAt":"2026-11-18T00:00:00Z"},"microcreditsCharged":10,"microcreditsFree":0,"replayed":false}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ATMEE_BASE_URL", srv.URL)
	return &bodies
}

// The two clip modes are checked before any request: --avatar and
// --transcript go together and refuse --image, --audio and --duration;
// without them --image and --prompt stay required.
func TestClipModeFlagsRefusedBeforeAnyRequest(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	bodies := clipAPI(t)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"--avatar", clipAvatar}, "--avatar and --transcript go together"},
		{[]string{"--transcript", "Hello"}, "--avatar and --transcript go together"},
		{[]string{"--avatar", clipAvatar, "--transcript", "Hello", "--image", "upload:a"}, "--image is not taken with --avatar"},
		{[]string{"--avatar", clipAvatar, "--transcript", "Hello", "--audio", "upload:v"}, "--audio is not taken with --avatar"},
		{[]string{"--avatar", clipAvatar, "--transcript", "Hello", "--duration", "6"}, "--duration is not taken with --avatar"},
		{[]string{"--avatar", clipAvatar, "--transcript", "  "}, "--transcript takes 1 to 1000 characters"},
		{[]string{"--avatar", clipAvatar, "--transcript", strings.Repeat("a", 1001)}, "--transcript takes 1 to 1000 characters"},
		{[]string{"--avatar", clipAvatar, "--transcript", "Hello", "--prompt", " "}, "--prompt takes 1 to 8000 characters"},
		{[]string{"--avatar", "Mira", "--transcript", "Hello"}, "is not an avatar id"},
		{[]string{"--prompt", "waves"}, "--image is required"},
		{[]string{"--image", "upload:a"}, "--prompt takes 1 to 8000 characters"},
		{[]string{"--image", "upload:a", "--prompt", "waves", "--audio", "upload:v"}, "--duration is required with --audio"},
	} {
		_, errOut, code := run(t, "", append([]string{"generate", "clip"}, c.args...)...)
		if code == 0 || !strings.Contains(errOut, c.want) {
			t.Errorf("%v: %d %s, want %q", c.args, code, errOut, c.want)
		}
	}
	if len(*bodies) != 0 {
		t.Fatalf("sent: %v", *bodies)
	}
}

// An avatar clip sends avatarId and transcript (and the prompt only when
// given), never image, audio or durationSeconds; an image clip sends no
// avatarId or transcript.
func TestClipRequestBodyPerMode(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	bodies := clipAPI(t)
	for _, args := range [][]string{
		{"--avatar", clipAvatar, "--transcript", "Good morning."},
		{"--avatar", clipAvatar, "--transcript", "Good morning.", "--prompt", "she waves", "--resolution", "720x1280", "--seed", "3"},
		{"--image", "upload:a", "--prompt", "a slow nod", "--audio", "upload:v", "--duration", "6"},
	} {
		if out, errOut, code := run(t, "", append([]string{"generate", "clip"}, args...)...); code != 0 || !strings.Contains(out, "0.10 credits charged") {
			t.Fatalf("%v: %d %s%s", args, code, out, errOut)
		}
	}
	want := []map[string]any{
		{"avatarId": clipAvatar, "transcript": "Good morning."},
		{"avatarId": clipAvatar, "transcript": "Good morning.", "prompt": "she waves",
			"resolution": map[string]any{"width": float64(720), "height": float64(1280)}, "seed": float64(3)},
		{"image": "upload:a", "prompt": "a slow nod", "audio": "upload:v", "durationSeconds": float64(6)},
	}
	if !reflect.DeepEqual(*bodies, want) {
		t.Fatalf("bodies:\n got %v\nwant %v", *bodies, want)
	}
}

// An avatar clip with a key retries as it is: there is no local file to
// upload again, so --idempotency-key is taken.
func TestAvatarClipTakesIdempotencyKey(t *testing.T) {
	isolate(t)
	t.Setenv("ATMEE_API_KEY", "sk_atmee_x")
	bodies := clipAPI(t)
	if _, errOut, code := run(t, "", "generate", "clip", "--avatar", clipAvatar, "--transcript", "Hi.",
		"--idempotency-key", "7c3e9a14-2b5d-4f6e-8a1c-0d9e8f7a6b5c"); code != 0 || len(*bodies) != 1 {
		t.Fatalf("%d %s %v", code, errOut, *bodies)
	}
}
