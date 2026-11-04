package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Atmanity/atmee-toolkit/cli/internal/avatarapi"
)

// CreateInput is a create request. Image, Voice and Knowledge are https URLs
// or upload:<uploadId> (what generation returns), or local paths (all of one
// kind). Image and Voice are optional: an avatar
// created without one is a draft, completed later by ReplaceAppearance and
// ReplaceVoice (a v1 avatar with only a portrait is render-only instead).
type CreateInput struct {
	Name           string
	Image          string
	Voice          string
	Knowledge      []string
	Engine         string // "v2" (default), "v1" or "v2hd" (Atmee V2 HD; "v2pro" is read as v2hd)
	Candidates     int    // v2: 1-10; 0 = API default (3)
	SeedSelection  string // v2: "manual" or "auto"
	PreviewMessage string
	Description    string
	Language       string
	Visibility     string
	Previews       *bool
	Facts          string
	Persona        map[string]any // API names: whoPrompt, whatPrompt, greeting, ...
	// Sponsor funds the avatar's pool on create (needs billing:write):
	// microcredits to move in, and the per-session cap.
	SponsorMicrocredits, SponsorMaxPerSession *int
	// RuntimeSeed is the v2 renderer's runtime seed (nil: the default, 42).
	RuntimeSeed *int
	// Resolution is the v2 or V2 HD frame size (nil: the engine's default,
	// 512x512 for v2, 640x640 for V2 HD).
	Resolution *Resolution
}

// Resolution is a v2 frame size in pixels.
type Resolution = avatarapi.RenderResolution

// The engines: v1, v2 and Atmee V2 HD (v2hd), a larger v2 model that
// builds, previews and serves like v2. EngineV2ProAlias is V2 HD's first
// name, which servers before the rename report and every server accepts.
const (
	EngineV1         = "v1"
	EngineV2         = "v2"
	EngineV2Pro      = "v2hd"
	EngineV2ProAlias = "v2pro"
)

// CanonicalEngine is an engine as given or reported, with V2 HD's first
// name (v2pro) read as v2hd.
func CanonicalEngine(e string) string {
	if e == EngineV2ProAlias {
		return EngineV2Pro
	}
	return e
}

// WireEngine is the engine as sent to the API: V2 HD goes as v2pro, the
// name every server accepts (servers before the rename refuse v2hd).
func WireEngine(e string) string {
	if CanonicalEngine(e) == EngineV2Pro {
		return EngineV2ProAlias
	}
	return e
}

// The account's v2 access (v2Access on GET /v1/account and on a
// redemption).
const (
	V2AccessGranted        = "granted"
	V2AccessInviteRequired = "invite_required"
	V2AccessUnavailable    = "unavailable"
)

// IsV2Family says the engine builds candidates and serves a take: v2 and
// V2 HD.
func IsV2Family(engine string) bool {
	e := CanonicalEngine(engine)
	return e == EngineV2 || e == EngineV2Pro
}

// The renderers' frame rule: each side a multiple of ResolutionStep from
// MinResolutionSide to MaxResolutionSide, at most MaxResolutionPixels in all
// for v2 (512x512) and MaxV2ProResolutionPixels for V2 HD (864x480).
const (
	ResolutionStep           = 32
	MinResolutionSide        = 256
	MaxResolutionSide        = 1024
	MaxResolutionPixels      = 512 * 512
	MaxV2ProResolutionPixels = 864 * 480
)

// parseWxH reads WIDTHxHEIGHT.
func parseWxH(s string) (w, h int, ok bool) {
	ws, hs, found := strings.Cut(strings.ToLower(strings.TrimSpace(s)), "x")
	wi, werr := strconv.Atoi(strings.TrimSpace(ws))
	hi, herr := strconv.Atoi(strings.TrimSpace(hs))
	return wi, hi, found && werr == nil && herr == nil
}

// ParseResolution reads WxH (e.g. 672x384) and checks it against the
// largest renderer's rule (V2 HD's); CheckResolutionFor checks an
// engine's own. Which sizes a deployment offers is the API's to say.
func ParseResolution(s string) (Resolution, error) {
	w, h, ok := parseWxH(s)
	if !ok {
		return Resolution{}, fmt.Errorf("--resolution takes WIDTHxHEIGHT, e.g. 512x512, 672x384 (16:9) or 384x672 (9:16); got %q", s)
	}
	r := Resolution{Width: w, Height: h}
	return r, CheckResolutionFor(EngineV2Pro, r)
}

// sizeRule is an engine's frame rule: its name in messages, its pixel cap
// (as WxH for messages, with examples), and the account limit listing the
// sizes a server offers it.
type sizeRule struct {
	label     string
	maxPixels int
	maxName   string
	examples  string
	offered   func(avatarapi.AccountLimits) *[]Resolution
}

var sizeRules = map[string]sizeRule{
	EngineV2: {label: "v2", maxPixels: MaxResolutionPixels, maxName: "512x512", examples: "672x384 or 384x672",
		offered: func(l avatarapi.AccountLimits) *[]Resolution { return l.V2Resolutions }},
	EngineV2Pro: {label: "V2 HD", maxPixels: MaxV2ProResolutionPixels, maxName: "864x480", examples: "640x640, 864x480 or 480x864",
		offered: func(l avatarapi.AccountLimits) *[]Resolution { return l.V2ProResolutions }},
}

// ruleFor is the engine's frame rule; any engine but V2 HD takes v2's.
func ruleFor(engine string) sizeRule {
	if r, ok := sizeRules[CanonicalEngine(engine)]; ok {
		return r
	}
	return sizeRules[EngineV2]
}

// CheckResolution refuses a frame size the v2 renderer cannot take.
func CheckResolution(r Resolution) error { return CheckResolutionFor(EngineV2, r) }

// CheckResolutionFor refuses a frame size the engine's renderer cannot take.
func CheckResolutionFor(engine string, r Resolution) error {
	side := func(n int) bool {
		return n >= MinResolutionSide && n <= MaxResolutionSide && n%ResolutionStep == 0
	}
	if !side(r.Width) || !side(r.Height) {
		return fmt.Errorf("resolution %dx%d: width and height must each be a multiple of %d from %d to %d",
			r.Width, r.Height, ResolutionStep, MinResolutionSide, MaxResolutionSide)
	}
	engine = CanonicalEngine(engine)
	rule, px := ruleFor(engine), r.Width*r.Height
	if px <= rule.maxPixels {
		return nil
	}
	err := fmt.Errorf("resolution %dx%d: %s frames are at most %d pixels (%s), e.g. %s",
		r.Width, r.Height, rule.label, rule.maxPixels, rule.maxName, rule.examples)
	if pro := sizeRules[EngineV2Pro]; engine != EngineV2Pro && px <= pro.maxPixels {
		return &V2ProSizeError{err}
	}
	return err
}

// V2ProSizeError is a frame size too large for v2 that V2 HD's rule
// allows. What to do about it depends on the command: a create can choose
// V2 HD; an existing avatar's engine cannot change.
type V2ProSizeError struct{ err error }

func (e *V2ProSizeError) Error() string { return e.err.Error() + "; that size is for V2 HD avatars" }

// OfferedResolutions are the frame sizes the server offers the engine
// (limits.v2ProResolutions for v2hd, else limits.v2Resolutions), or nil
// where it lists none (every size the rule allows).
func OfferedResolutions(acct *Account, engine string) []Resolution {
	if acct == nil {
		return nil
	}
	if rs := ruleFor(engine).offered(acct.Limits); rs != nil {
		return *rs
	}
	return nil
}

// CheckOffered refuses a frame size the server does not offer the engine,
// naming the ones it does. Without a list (an older server, or one that
// offers every size the rule allows) it lets the server decide: the server
// is the authority either way.
func CheckOffered(acct *Account, engine string, r Resolution) error {
	offered := OfferedResolutions(acct, engine)
	if len(offered) == 0 {
		return nil
	}
	names := make([]string, len(offered))
	for i, o := range offered {
		if o.Width == r.Width && o.Height == r.Height {
			return nil
		}
		names[i] = fmt.Sprintf("%dx%d", o.Width, o.Height)
	}
	return fmt.Errorf("resolution %dx%d is not a %s size on this server; offered: %s (`atmee account` lists them)",
		r.Width, r.Height, ruleFor(engine).label, strings.Join(names, ", "))
}

// CheckSize checks a frame size for an engine before it is sent: the
// engine's rule, then the sizes the account lists for it. A v1 engine is
// left to the server (which refuses it), and so is the list when the
// account cannot be read: the server is the authority.
func (c *Client) CheckSize(ctx context.Context, engine string, r Resolution) error {
	if !IsV2Family(engine) {
		return nil
	}
	if err := CheckResolutionFor(engine, r); err != nil {
		return err
	}
	acct, err := c.Account(ctx)
	if err != nil {
		return nil
	}
	return CheckOffered(acct, engine, r)
}

// MaxRuntimeSeed is the largest runtime seed the API takes.
const MaxRuntimeSeed = 1<<31 - 1

// CheckRuntimeSeed refuses a runtime seed the API would refuse.
func CheckRuntimeSeed(seed int) error {
	if seed < 0 || seed > MaxRuntimeSeed {
		return fmt.Errorf("--runtime-seed must be from 0 to %d", MaxRuntimeSeed)
	}
	return nil
}

// CheckPrimeText refuses a v2 preview message the API would refuse.
func CheckPrimeText(engine, text string) error {
	if IsV2Family(engine) && runeLen(text) > MaxPrimeTextChars {
		return fmt.Errorf("v2 preview messages are spoken as a ~30 s clip: at most %d characters (got %d)", MaxPrimeTextChars, runeLen(text))
	}
	return nil
}

func runeLen(s string) int { return utf8.RuneCountInString(strings.TrimSpace(s)) }

func engineOf(a *Avatar) string {
	if a.Engine == nil {
		return EngineV1
	}
	return CanonicalEngine(string(*a.Engine))
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "https://") || strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "upload:")
}

// IsRemote says s names a file the API already can reach (an https URL or
// upload:<uploadId>) rather than a local path.
func IsRemote(s string) bool { return isURL(s) }

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// Manifest is the avatar.json of a create.
func (in CreateInput) Manifest() (map[string]any, error) {
	engine := CanonicalEngine(in.Engine)
	if engine == "" {
		engine = EngineV2
	}
	if IsV2Family(engine) && (in.Image == "" || in.Voice == "") && (in.Candidates != 0 || in.SeedSelection != "") {
		return nil, errors.New("--candidates and --seed-selection need both --image and --voice: a v2 avatar created without them " +
			"builds one candidate, used as soon as it is finished, once both are given")
	}
	if in.Candidates != 0 && (in.Candidates < 1 || in.Candidates > MaxCandidatesPerRequest) {
		return nil, fmt.Errorf("--candidates must be 1-%d", MaxCandidatesPerRequest)
	}
	if err := CheckPrimeText(engine, in.PreviewMessage); err != nil {
		return nil, err
	}
	m := map[string]any{"schemaVersion": 1, "name": in.Name}
	set := func(k, v string) {
		if v != "" {
			m[k] = v
		}
	}
	set("description", in.Description)
	set("language", in.Language)
	set("visibility", in.Visibility)
	set("previewMessage", in.PreviewMessage)
	set("seedSelection", in.SeedSelection)
	if in.Previews != nil {
		m["previews"] = *in.Previews
	}
	if in.RuntimeSeed != nil {
		if err := CheckRuntimeSeed(*in.RuntimeSeed); err != nil {
			return nil, err
		}
		m["runtimeSeed"] = *in.RuntimeSeed
	}
	if r := in.Resolution; r != nil {
		if engine == EngineV1 {
			return nil, errors.New("--resolution is only for v2 and V2 HD avatars")
		}
		if err := CheckResolutionFor(engine, *r); err != nil {
			return nil, err
		}
		m["resolution"] = map[string]int{"width": r.Width, "height": r.Height}
	}
	if engine != EngineV1 {
		m["engine"] = WireEngine(engine)
	}
	if in.SponsorMicrocredits != nil || in.SponsorMaxPerSession != nil {
		sp := map[string]int{}
		if v := in.SponsorMicrocredits; v != nil {
			if *v <= 0 {
				return nil, errors.New("--sponsor-microcredits must be positive")
			}
			sp["microcredits"] = *v
		}
		if v := in.SponsorMaxPerSession; v != nil {
			if *v <= 0 {
				return nil, errors.New("--sponsor-max-per-session must be positive")
			}
			sp["maxPerSession"] = *v
		}
		m["sponsor"] = sp
	}
	if in.Candidates != 0 {
		m["seedPreviews"] = map[string]int{"count": in.Candidates}
	}
	if len(in.Persona) > 0 {
		m["persona"] = in.Persona
	}
	if in.Facts != "" {
		m["knowledge"] = map[string]string{"facts": in.Facts}
	}
	return m, nil
}

// Create creates an avatar. It is sent once: a transport failure is not
// resent (see sentOnce).
func (c *Client) Create(ctx context.Context, in CreateInput) (*CreateResult, *Quota, error) {
	manifest, err := in.Manifest()
	if err != nil {
		return nil, nil, err
	}
	var files []string
	for _, f := range append([]string{in.Image, in.Voice}, in.Knowledge...) {
		if f != "" {
			files = append(files, f)
		}
	}
	urls := 0
	for _, f := range files {
		if strings.HasPrefix(f, "http://") {
			return nil, nil, fmt.Errorf("asset URLs must be https:// (%s)", f)
		}
		if isURL(f) {
			urls++
		}
	}
	var (
		contentType string
		payload     []byte
	)
	switch {
	case len(files) == 0:
		// A draft with nothing to upload: the manifest alone.
		contentType = "application/json"
		payload, _ = json.Marshal(manifest)
	case urls == len(files):
		assets := map[string]any{}
		if in.Image != "" {
			assets["image"] = map[string]string{"url": in.Image}
		}
		if in.Voice != "" {
			assets["voice"] = map[string]string{"url": in.Voice}
		}
		if len(in.Knowledge) > 0 {
			k := make([]map[string]string, len(in.Knowledge))
			for i, u := range in.Knowledge {
				k[i] = map[string]string{"url": u}
			}
			assets["knowledge"] = k
		}
		manifest["assets"] = assets
		contentType = "application/json"
		payload, _ = json.Marshal(manifest)
	case urls == 0:
		zipped, err := bundleZip(manifest, in.Image, in.Voice, in.Knowledge)
		if err != nil {
			return nil, nil, err
		}
		contentType, payload, err = zipUpload(zipped)
		if err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, errors.New("give image, voice and knowledge either all as https URLs or upload: references, or all as local files")
	}
	r, err := c.gen.CreateAvatarWithBodyWithResponse(ctx, contentType, bytes.NewReader(payload))
	if r == nil {
		return nil, nil, sentOnce(err, "check `atmee avatars list`")
	}
	res, err := decode[CreateResult](r.HTTPResponse, r.Body, err)
	if err != nil {
		return nil, nil, err
	}
	return res, quotaOf(r.HTTPResponse.Header), nil
}

// zipDate keeps a bundle of the same files byte-identical from run to run.
var zipDate = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

// bundleZip is avatar.json + image/ + voice/ + knowledge/ (each optional).
func bundleZip(manifest map[string]any, image, voice string, knowledge []string) ([]byte, error) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	add := func(name string, data []byte) error {
		w, err := z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: zipDate})
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	mj, _ := json.Marshal(manifest)
	if err := add("avatar.json", mj); err != nil {
		return nil, err
	}
	addFile := func(folder, path string) error {
		data, err := os.ReadFile(expandHome(path))
		if err != nil {
			return fmt.Errorf("%s: file not found", path)
		}
		return add(folder+"/"+filepath.Base(path), data)
	}
	if image != "" {
		if err := addFile("image", image); err != nil {
			return nil, err
		}
	}
	if voice != "" {
		if err := addFile("voice", voice); err != nil {
			return nil, err
		}
	}
	for _, k := range knowledge {
		if err := addFile("knowledge", k); err != nil {
			return nil, err
		}
	}
	if err := z.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// zipUpload wraps the bundle as multipart field `file` with a fixed boundary,
// so a retry sends the same bytes.
func zipUpload(zipped []byte) (string, []byte, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if err := w.SetBoundary("atmee-avatar-bundle"); err != nil {
		return "", nil, err
	}
	fw, err := w.CreateFormFile("file", "avatar.zip")
	if err != nil {
		return "", nil, err
	}
	if _, err := fw.Write(zipped); err != nil {
		return "", nil, err
	}
	if err := w.Close(); err != nil {
		return "", nil, err
	}
	return w.FormDataContentType(), buf.Bytes(), nil
}

// defaultTimeout gives requests without their own deadline a timeout, so the
// long join waits can set a longer one.
type defaultTimeout struct {
	base http.RoundTripper
	d    time.Duration
}

func (t defaultTimeout) RoundTrip(req *http.Request) (*http.Response, error) {
	if _, ok := req.Context().Deadline(); ok || t.d <= 0 {
		return t.base.RoundTrip(req)
	}
	ctx, cancel := context.WithTimeout(req.Context(), t.d)
	resp, err := t.base.RoundTrip(req.WithContext(ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	resp.Body = cancelOnClose{resp.Body, cancel}
	return resp, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}
