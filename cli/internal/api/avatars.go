package api

import (
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
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Atmanity/toolkit/cli/internal/avatarapi"
	"github.com/google/uuid"
)

// Generated model aliases the commands use.
type (
	Avatar          = avatarapi.AvatarDetail
	AvatarSummary   = avatarapi.AvatarSummary
	CreateResult    = avatarapi.AvatarCreateResponse
	CandidateList   = avatarapi.CandidateListResponse
	Candidate       = avatarapi.Candidate
	Sponsor         = avatarapi.AvatarSponsor
	SponsorGrant    = avatarapi.SponsorGrantResponse
	SponsorWithdraw = avatarapi.SponsorWithdrawResponse
	AvatarSession   = avatarapi.AvatarSessionCreateResponse
	SessionStatus   = avatarapi.AvatarSessionStatusResponse
	SessionEnd      = avatarapi.AvatarSessionEndResponse
	EmbedSession    = avatarapi.EmbedSessionStartResponse
	BuildStage      = avatarapi.BuildStage
	Knowledge       = avatarapi.AvatarKnowledge
	SourceFile      = avatarapi.AvatarSourceFile
	VoiceFile       = avatarapi.AvatarVoice
	Monetization    = avatarapi.AvatarMonetization
	VersionDeleted  = avatarapi.VersionDeleteResponse
)

// decode checks the status and unmarshals any 2xx body into T.
func decode[T any](resp *http.Response, body []byte, err error) (*T, error) {
	if err != nil {
		return nil, err
	}
	if err := check(resp, body); err != nil {
		return nil, err
	}
	out := new(T)
	if len(bytes.TrimSpace(body)) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return nil, fmt.Errorf("unexpected response: %w", err)
		}
	}
	return out, nil
}

// ParseID validates an avatar or session id before any request.
func ParseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return id, fmt.Errorf("%q is not an avatar or session id (a UUID)", s)
	}
	return id, nil
}

// ParseCandidateID validates a candidate id before any request. Any UUID
// spelling (upper case, braces) is accepted; the API sees the canonical one.
func ParseCandidateID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return id, fmt.Errorf("%q is not a candidate id (a UUID)", s)
	}
	return id, nil
}

func jsonBody(v any) io.Reader {
	b, _ := json.Marshal(v)
	return bytes.NewReader(b)
}

// --- read ---

func (c *Client) Get(ctx context.Context, avatarID string) (*Avatar, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.GetAvatarWithResponse(ctx, id)
	if r == nil {
		return nil, err
	}
	return decode[Avatar](r.HTTPResponse, r.Body, err)
}

// Knowledge reads the facts text, the questionnaire answers and the
// knowledge files (with presigned URLs).
func (c *Client) Knowledge(ctx context.Context, avatarID string) (*Knowledge, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.GetAvatarKnowledgeWithResponse(ctx, id)
	if r == nil {
		return nil, err
	}
	return decode[Knowledge](r.HTTPResponse, r.Body, err)
}

// Monetization reads the avatar's monetization eligibility and earnings.
func (c *Client) Monetization(ctx context.Context, avatarID string) (*Monetization, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.GetAvatarMonetizationWithResponse(ctx, id)
	if r == nil {
		return nil, err
	}
	return decode[Monetization](r.HTTPResponse, r.Body, err)
}

// SetThumbnail uploads a local image (jpg, png or webp) as the avatar's
// Explore thumbnail and returns the detail. Nothing is rebuilt.
func (c *Client) SetThumbnail(ctx context.Context, avatarID, path string) (*Avatar, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	ct, body, err := multipartFiles("file", []string{path}, nil)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.ReplaceAvatarThumbnailWithBodyWithResponse(ctx, id, ct, body)
	if r == nil {
		return nil, err
	}
	return decode[Avatar](r.HTTPResponse, r.Body, err)
}

// RemoveThumbnail removes the Explore thumbnail (Explore shows the portrait).
func (c *Client) RemoveThumbnail(ctx context.Context, avatarID string) (*Avatar, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.DeleteAvatarThumbnailWithResponse(ctx, id)
	if r == nil {
		return nil, err
	}
	return decode[Avatar](r.HTTPResponse, r.Body, err)
}

// Appearance reads the portrait last given, as a presigned URL.
func (c *Client) Appearance(ctx context.Context, avatarID string) (*SourceFile, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.GetAvatarAppearanceWithResponse(ctx, id)
	if r == nil {
		return nil, err
	}
	return decode[SourceFile](r.HTTPResponse, r.Body, err)
}

// Voice reads the voice sample last given, as a presigned URL, and the
// voice engine.
func (c *Client) Voice(ctx context.Context, avatarID string) (*VoiceFile, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.GetAvatarVoiceWithResponse(ctx, id)
	if r == nil {
		return nil, err
	}
	return decode[VoiceFile](r.HTTPResponse, r.Body, err)
}

// List returns one page, newest first, and the cursor for the next (or nil).
func (c *Client) List(ctx context.Context, limit int, before *time.Time) ([]AvatarSummary, *time.Time, error) {
	r, err := c.gen.ListAvatarsWithResponse(ctx, &avatarapi.ListAvatarsParams{Limit: &limit, Before: before})
	if r == nil {
		return nil, nil, err
	}
	page, err := decode[avatarapi.AvatarListResponse](r.HTTPResponse, r.Body, err)
	if err != nil {
		return nil, nil, err
	}
	return page.Items, page.NextBefore, nil
}

// --- write ---

// Update PATCHes exactly the given fields (API names). The API leaves a null
// or absent field unchanged and removes a value sent empty ("" or []).
func (c *Client) Update(ctx context.Context, avatarID string, fields map[string]any) (*Avatar, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("nothing to update")
	}
	if msg, ok := fields["previewMessage"].(string); ok && runeLen(msg) > MaxPrimeTextChars {
		// Only v2 has this limit: look the engine up before refusing.
		a, err := c.Get(ctx, avatarID)
		if err != nil {
			return nil, err
		}
		if err := CheckPrimeText(engineOf(a), msg); err != nil {
			return nil, err
		}
	}
	r, err := c.gen.UpdateAvatarWithBodyWithResponse(ctx, id, "application/json", jsonBody(fields))
	if r == nil {
		return nil, err
	}
	return decode[Avatar](r.HTTPResponse, r.Body, err)
}

func (c *Client) Delete(ctx context.Context, avatarID string) error {
	id, err := ParseID(avatarID)
	if err != nil {
		return err
	}
	r, err := c.gen.DeleteAvatarWithResponse(ctx, id)
	if r == nil {
		return err
	}
	_, err = decode[struct{}](r.HTTPResponse, r.Body, err)
	return err
}

// DeleteVersion deletes an older version of a v2 avatar: every take of it and
// the files only it used. pendingFiles counts what storage could not delete
// yet; repeating the call retries them.
func (c *Client) DeleteVersion(ctx context.Context, avatarID, versionID string) (*VersionDeleted, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.DeleteAvatarVersionWithResponse(ctx, id, versionID)
	if r == nil {
		return nil, err
	}
	return decode[VersionDeleted](r.HTTPResponse, r.Body, err)
}

// ReplaceAppearance sets a new portrait from a local file, an https URL or
// upload:<uploadId> (v2: new candidates are captured).
func (c *Client) ReplaceAppearance(ctx context.Context, avatarID, path string) error {
	return c.putFile(avatarID, path, "image", func(id uuid.UUID, ct string, body io.Reader) (*http.Response, []byte, error) {
		r, err := c.gen.ReplaceAvatarAppearanceWithBodyWithResponse(ctx, id, ct, body)
		if r == nil {
			return nil, nil, err
		}
		return r.HTTPResponse, r.Body, err
	})
}

// ReplaceVoice sets a new voice sample from a local file, an https URL or
// upload:<uploadId> (v2: re-cloned, new candidates).
func (c *Client) ReplaceVoice(ctx context.Context, avatarID, path string) error {
	return c.putFile(avatarID, path, "voice", func(id uuid.UUID, ct string, body io.Reader) (*http.Response, []byte, error) {
		r, err := c.gen.ReplaceAvatarVoiceWithBodyWithResponse(ctx, id, ct, body)
		if r == nil {
			return nil, nil, err
		}
		return r.HTTPResponse, r.Body, err
	})
}

// putFile sends a local path as a file, or a remote one as the JSON form's
// field.
func (c *Client) putFile(avatarID, path, field string, send func(uuid.UUID, string, io.Reader) (*http.Response, []byte, error)) error {
	id, err := ParseID(avatarID)
	if err != nil {
		return err
	}
	if strings.HasPrefix(path, "http://") {
		return fmt.Errorf("asset URLs must be https:// (%s)", path)
	}
	if isURL(path) {
		resp, b, err := send(id, "application/json", replaceFromRef(field, path))
		_, err = decode[struct{}](resp, b, err)
		return err
	}
	ct, body, err := multipartFiles("file", []string{path}, nil)
	if err != nil {
		return err
	}
	resp, b, err := send(id, ct, body)
	_, err = decode[struct{}](resp, b, err)
	return err
}

// ReplaceKnowledge replaces the whole knowledge file set; no paths clears it.
func (c *Client) ReplaceKnowledge(ctx context.Context, avatarID string, paths []string) error {
	id, err := ParseID(avatarID)
	if err != nil {
		return err
	}
	// descriptions keeps an explicit clear a multipart body, which the
	// endpoint requires.
	ct, body, err := multipartFiles("files", paths, map[string]string{"descriptions": "{}"})
	if err != nil {
		return err
	}
	r, err := c.gen.ReplaceAvatarKnowledgeWithBodyWithResponse(ctx, id, ct, body)
	if r == nil {
		return err
	}
	_, err = decode[struct{}](r.HTTPResponse, r.Body, err)
	return err
}

func multipartFiles(field string, paths []string, fields map[string]string) (string, io.Reader, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, p := range paths {
		data, err := os.ReadFile(expandHome(p))
		if err != nil {
			return "", nil, fmt.Errorf("%s: not a readable local file", p)
		}
		fw, err := w.CreateFormFile(field, filepath.Base(p))
		if err != nil {
			return "", nil, err
		}
		if _, err := fw.Write(data); err != nil {
			return "", nil, err
		}
	}
	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return "", nil, err
		}
	}
	if err := w.Close(); err != nil {
		return "", nil, err
	}
	return w.FormDataContentType(), &buf, nil
}

// --- v2 candidates ---

// CandidatesPage is a candidate list with the account's capture quota.
type CandidatesPage struct {
	*CandidateList
	Quota *Quota `json:"quota,omitempty"`
}

// Candidates lists the current version's candidates and the serving one.
func (c *Client) Candidates(ctx context.Context, avatarID string) (*CandidatesPage, error) {
	return c.candidates(ctx, avatarID, nil) // versions=current is the default
}

// AllCandidates lists the candidates of every version: an older version's
// completed take (restorable) can be selected to go back to that version.
func (c *Client) AllCandidates(ctx context.Context, avatarID string) (*CandidatesPage, error) {
	all := avatarapi.CandidateVersionsAll
	return c.candidates(ctx, avatarID, &avatarapi.ListAvatarCandidatesParams{Versions: &all})
}

func (c *Client) candidates(ctx context.Context, avatarID string, params *avatarapi.ListAvatarCandidatesParams) (*CandidatesPage, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.ListAvatarCandidatesWithResponse(ctx, id, params)
	if r == nil {
		return nil, err
	}
	cl, err := decode[CandidateList](r.HTTPResponse, r.Body, err)
	if err != nil {
		return nil, err
	}
	return &CandidatesPage{CandidateList: cl, Quota: quotaOf(r.HTTPResponse.Header)}, nil
}

// AddCandidates captures count (1-10) more candidates. It is sent once: a
// transport failure is not resent (see sentOnce).
func (c *Client) AddCandidates(ctx context.Context, avatarID string, count int) (*CandidatesPage, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	if count < 1 || count > MaxCandidatesPerRequest {
		return nil, fmt.Errorf("count must be 1-%d", MaxCandidatesPerRequest)
	}
	r, err := c.gen.CreateAvatarCandidatesWithBodyWithResponse(ctx, id,
		"application/json", jsonBody(map[string]int{"count": count}))
	if r == nil {
		return nil, sentOnce(err, "check `atmee avatars candidates list "+avatarID+"`")
	}
	cl, err := decode[CandidateList](r.HTTPResponse, r.Body, err)
	if err != nil {
		return nil, err
	}
	return &CandidatesPage{CandidateList: cl, Quota: quotaOf(r.HTTPResponse.Header)}, nil
}

// SelectCandidate makes a completed candidate live. A completed take of an
// older version (restorable) goes back to that version: its portrait, voice
// sample and preview message become the avatar's again.
func (c *Client) SelectCandidate(ctx context.Context, avatarID, candidateID string) (*Avatar, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	cid, err := ParseCandidateID(candidateID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.SelectAvatarCandidateWithBodyWithResponse(ctx, id, "application/json",
		jsonBody(map[string]string{"candidateId": cid.String()}))
	if r == nil {
		return nil, err
	}
	return decode[Avatar](r.HTTPResponse, r.Body, err)
}

// --- sponsor ---

func (c *Client) Sponsor(ctx context.Context, avatarID string) (*Sponsor, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.GetAvatarSponsorWithResponse(ctx, id)
	if r == nil {
		return nil, err
	}
	return decode[Sponsor](r.HTTPResponse, r.Body, err)
}

// UpdateSponsor PATCHes the given settings (API names); nil clears a cap.
func (c *Client) UpdateSponsor(ctx context.Context, avatarID string, settings map[string]any) (*Sponsor, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	if len(settings) == 0 {
		return nil, fmt.Errorf("nothing to update")
	}
	r, err := c.gen.UpdateAvatarSponsorWithBodyWithResponse(ctx, id, "application/json", jsonBody(settings))
	if r == nil {
		return nil, err
	}
	return decode[Sponsor](r.HTTPResponse, r.Body, err)
}

// GrantSponsor moves account credits into the pool; a key makes a retry pay once.
func (c *Client) GrantSponsor(ctx context.Context, avatarID string, microcredits int, key string) (*SponsorGrant, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	// Always keyed: a retry after a lost response must never pay twice.
	return idempotent(c, key, func(key string) (*SponsorGrant, error) {
		r, err := c.gen.GrantAvatarSponsorWithBodyWithResponse(ctx, id,
			&avatarapi.GrantAvatarSponsorParams{IdempotencyKey: &key}, "application/json",
			jsonBody(map[string]int{"microcredits": microcredits}))
		if r == nil {
			return nil, err
		}
		return decode[SponsorGrant](r.HTTPResponse, r.Body, err)
	})
}

// WithdrawSponsor returns unreserved pool credits (all when microcredits is nil).
func (c *Client) WithdrawSponsor(ctx context.Context, avatarID string, microcredits *int) (*SponsorWithdraw, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	body := map[string]int{}
	if microcredits != nil {
		body["microcredits"] = *microcredits
	}
	r, err := c.gen.WithdrawAvatarSponsorWithBodyWithResponse(ctx, id, "application/json", jsonBody(body))
	if r == nil {
		return nil, err
	}
	return decode[SponsorWithdraw](r.HTTPResponse, r.Body, err)
}

// --- sessions ---

// AvatarSessionInput starts an avatar-only session in your LiveKit room.
type AvatarSessionInput struct {
	LiveKitURL, LiveKitToken, AgentIdentity string
	MaxDurationSeconds                      *int
	Metadata                                map[string]any
	// WaitJoined returns once the avatar is in the room (minutes on a cold v2 pool).
	WaitJoined bool
}

func (c *Client) StartAvatarSession(ctx context.Context, avatarID string, in AvatarSessionInput) (*AvatarSession, error) {
	id, err := ParseID(avatarID)
	if err != nil {
		return nil, err
	}
	body := map[string]any{"livekitUrl": in.LiveKitURL, "livekitToken": in.LiveKitToken}
	if in.AgentIdentity != "" {
		body["agentIdentity"] = in.AgentIdentity
	}
	if in.MaxDurationSeconds != nil {
		body["maxDurationSeconds"] = *in.MaxDurationSeconds
	}
	if in.Metadata != nil {
		body["metadata"] = in.Metadata
	}
	var params avatarapi.CreateAvatarSessionParams
	if in.WaitJoined {
		w := avatarapi.CreateAvatarSessionParamsWaitFor("avatar_joined")
		params.WaitFor = &w
	}
	return retryWarming(ctx, c, avatarID, func() (*AvatarSession, error) {
		// Each attempt gets the whole join wait: time spent waiting out
		// avatar_warming must not cut short the start that gets through.
		actx := ctx
		if in.WaitJoined {
			var cancel context.CancelFunc
			actx, cancel = context.WithTimeout(ctx, c.JoinTimeout)
			defer cancel()
		}
		r, err := c.gen.CreateAvatarSessionWithBodyWithResponse(actx, id, &params, "application/json", jsonBody(body))
		if r == nil {
			return nil, err
		}
		return decode[AvatarSession](r.HTTPResponse, r.Body, err)
	})
}

func (c *Client) AvatarSessionStatus(ctx context.Context, sessionID string) (*SessionStatus, error) {
	id, err := ParseID(sessionID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.GetAvatarSessionWithResponse(ctx, id)
	if r == nil {
		return nil, err
	}
	return decode[SessionStatus](r.HTTPResponse, r.Body, err)
}

// EndAvatarSession is idempotent: ending an ended session reports alreadyEnded.
func (c *Client) EndAvatarSession(ctx context.Context, sessionID string) (*SessionEnd, error) {
	id, err := ParseID(sessionID)
	if err != nil {
		return nil, err
	}
	r, err := c.gen.EndAvatarSessionWithResponse(ctx, id)
	if r == nil {
		return nil, err
	}
	return decode[SessionEnd](r.HTTPResponse, r.Body, err)
}

// EmbedSessionInput starts a full Atmee conversation (our LLM, voice and avatar).
type EmbedSessionInput struct {
	MaxDurationSeconds *int
	// VisitorID unlocks that visitor's memory across sessions: it MUST be an
	// unguessable id your server derives, never one from the browser.
	VisitorID string
	Language  string
	Metadata  map[string]any
	Location  map[string]any
	// Tools the avatar may call in this conversation (see ParseTools), sent
	// as appTools.tools.
	Tools Tools
	// Instructions is one text for all of Tools, saying how and when the
	// avatar uses them (and any app persona or rules), sent as
	// appTools.instructions; nil leaves it out. It needs Tools.
	Instructions *string
	// NoWelcome starts without the avatar's self-introduction or welcome
	// back, sent as welcome: false; it then waits for the user to speak.
	NoWelcome bool
	// CheckInAfterS is the seconds of silence before the avatar checks in, 0
	// to 3600, 0 for no check-in; nil keeps the default.
	CheckInAfterS *float64
}

// StartSession waits for the avatar to join, which can take minutes on a cold pool.
func (c *Client) StartSession(ctx context.Context, avatarID string, in EmbedSessionInput) (*EmbedSession, error) {
	body := map[string]any{"avatarId": avatarID}
	if in.MaxDurationSeconds != nil {
		body["maxDurationSeconds"] = *in.MaxDurationSeconds
	}
	if in.VisitorID != "" {
		if n := utf8.RuneCountInString(in.VisitorID); n < 16 || n > 256 {
			return nil, fmt.Errorf("visitor id must be 16-256 characters (got %d): use an unguessable id your server derives", n)
		}
		body["visitorId"] = in.VisitorID
	}
	if in.Language != "" {
		body["language"] = in.Language
	}
	if in.Metadata != nil {
		body["metadata"] = in.Metadata
	}
	if in.Location != nil {
		body["location"] = in.Location
	}
	if in.Tools != nil {
		appTools := map[string]any{"tools": in.Tools}
		if in.Instructions != nil {
			appTools["instructions"] = *in.Instructions
		}
		body["appTools"] = appTools
	}
	if in.NoWelcome {
		body["welcome"] = false
	}
	if in.CheckInAfterS != nil {
		seconds := *in.CheckInAfterS
		if !(seconds >= 0 && seconds <= 3600) {
			return nil, fmt.Errorf("check-in must be 0 to 3600 seconds, 0 for none (got %v)", seconds)
		}
		body["checkInAfterS"] = seconds
	}
	ctx, cancel := context.WithTimeout(ctx, JoinWaitTimeout)
	defer cancel()
	r, err := c.gen.StartEmbedSessionWithBodyWithResponse(ctx, "application/json", jsonBody(body))
	if r == nil {
		return nil, err
	}
	return decode[EmbedSession](r.HTTPResponse, r.Body, err)
}

// retryWarming sends an avatar-only session start of a v2 avatar again while
// the API answers 503 avatar_warming: right after a v2 publish, a rendering
// node first fetches the avatar's bundle. On the first such answer it reads
// the avatar once; when that avatar is not v2, or the read fails (a key
// without avatars:read), it returns the answer at once. It waits Retry-After
// seconds (at least 5; 30 when the header is missing) and gives up with that
// answer once the next wait would end past WarmingTimeout (counted from the
// first send). An attempt that waits for the join has its own JoinTimeout on
// top. Cancelling ctx during a wait returns ctx's error at once. Every other
// answer, no_capacity included, returns at once.
func retryWarming[T any](ctx context.Context, c *Client, avatarID string, call func() (*T, error)) (*T, error) {
	deadline := c.now().Add(c.WarmingTimeout)
	isV2 := false
	for {
		out, err := call()
		var apiErr *Error
		if !errors.As(err, &apiErr) || apiErr.Status != http.StatusServiceUnavailable || apiErr.Code != "avatar_warming" {
			return out, err
		}
		if !isV2 {
			if a, gerr := c.Get(ctx, avatarID); gerr != nil || engineOf(a) != "v2" {
				return out, err
			}
			isV2 = true
		}
		wait := 30 * time.Second
		if apiErr.RetryAfter > 0 {
			wait = max(time.Duration(apiErr.RetryAfter)*time.Second, 5*time.Second)
		}
		if c.now().Add(wait).After(deadline) {
			return out, err
		}
		if c.OnWarming != nil {
			c.OnWarming(wait)
		}
		if werr := c.sleepCtx(ctx, wait); werr != nil {
			return nil, werr
		}
	}
}
