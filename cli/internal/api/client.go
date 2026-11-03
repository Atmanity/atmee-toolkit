// Package api wraps the generated Avatar API client with what every command
// needs: the API key or sign-in, error mapping, X-RateLimit quota, and calls that start
// GPU work or pay sent without blind resends.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Atmanity/atmee-toolkit/cli/internal/avatarapi"
	"github.com/google/uuid"
)

// DefaultBaseURL is production; ATMEE_BASE_URL points the CLI elsewhere, such
// as the local mock http://localhost:8091.
const DefaultBaseURL = "https://api.atmanity.us"

const (
	// MaxCandidatesPerRequest mirrors the API so mistakes fail before a request.
	MaxCandidatesPerRequest = 10
	// MaxPrimeTextChars bounds a v2 preview message (Unicode code points; ~30 s).
	MaxPrimeTextChars = 300
	// JoinWaitTimeout covers a cold v2 pool before the avatar joins a room,
	// and the retries while it warms up.
	JoinWaitTimeout = 10 * time.Minute

	inProgressWaits = 15
)

// Client is the Avatar API for one key or signed-in user.
type Client struct {
	BaseURL string
	gen     *avatarapi.ClientWithResponses
	// http and sign are what gen sends with, for clients of routes outside
	// the generated Avatar API client.
	http *http.Client
	sign func(context.Context, *http.Request) error
	// Sleep is time.Sleep; tests replace it.
	Sleep func(time.Duration)
	// InProgressSleep is the pause before re-sending a request whose first
	// attempt is still running (409 idempotency_key_in_progress).
	InProgressSleep time.Duration
	// MockScenario is sent as X-Mock-Scenario on every request (the local
	// mock's failure injection; production ignores it).
	MockScenario string
	// JoinTimeout bounds each attempt of an avatar-only session start that
	// waits for the join; New sets JoinWaitTimeout.
	JoinTimeout time.Duration
	// WarmingTimeout bounds the resends of an avatar-only start while a v2
	// avatar warms up, counted from the first send. It is a budget of its
	// own: an attempt sent near its end still gets all of JoinTimeout to
	// join. New sets JoinWaitTimeout.
	WarmingTimeout time.Duration
	// OnWarming, when set, hears each wait before an avatar-only start of a
	// v2 avatar is sent again after 503 avatar_warming.
	OnWarming func(wait time.Duration)
	// now is time.Now; tests replace it.
	now func() time.Time
	// sleepCtx is the warming wait: it ends early with ctx's error once ctx
	// is done, so a cancelled start does not sit out a Retry-After. Tests
	// replace it.
	sleepCtx func(ctx context.Context, d time.Duration) error
}

// sleepCtx waits d, or until ctx is done (returning ctx's error).
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Credential signs each request: an API key, or a signed-in user's token.
type Credential interface {
	Authorize(ctx context.Context, r *http.Request) error
}

// APIKey sends the key as X-Api-Key.
type APIKey string

func (k APIKey) Authorize(_ context.Context, r *http.Request) error {
	r.Header.Set("X-Api-Key", string(k))
	return nil
}

// New returns a client for baseURL that signs requests with cred.
func New(baseURL string, cred Credential, timeout time.Duration) (*Client, error) {
	if k, ok := cred.(APIKey); cred == nil || (ok && strings.TrimSpace(string(k)) == "") {
		return nil, errors.New("not signed in: run `atmee login`, or set ATMEE_API_KEY (create one in Atmee Studio -> API keys)")
	}
	baseURL = strings.TrimRight(baseURL, "/")
	c := &Client{BaseURL: baseURL, Sleep: time.Sleep, InProgressSleep: 2 * time.Second, JoinTimeout: JoinWaitTimeout, WarmingTimeout: JoinWaitTimeout, now: time.Now, sleepCtx: sleepCtx}
	c.http = &http.Client{Transport: defaultTimeout{http.DefaultTransport, timeout}}
	c.sign = func(ctx context.Context, r *http.Request) error {
		if err := cred.Authorize(ctx, r); err != nil {
			return err
		}
		if c.MockScenario != "" {
			r.Header.Set("X-Mock-Scenario", c.MockScenario)
		}
		return nil
	}
	gen, err := avatarapi.NewClientWithResponses(baseURL,
		avatarapi.WithHTTPClient(c.http), avatarapi.WithRequestEditorFn(c.sign))
	if err != nil {
		return nil, err
	}
	c.gen = gen
	return c, nil
}

// Error is a non-2xx answer from the Avatar API.
type Error struct {
	Status  int
	Code    string
	Message string
	// Set on 409 (request_in_progress), 429 rate_limited (reason pending_cap)
	// and 503: the seconds from Retry-After.
	Reason     string
	RetryAfter int
	// What a refused share, statement or monetization change found (see
	// ProfileRefusal in the spec): the fields public still needs, the
	// plan's cap on monetized avatars and how many are, the payout
	// account's status, the sponsor pool's credits, the moderation state.
	Missing                 []string
	Cap, MonetizingAvatars  *int
	PayoutsStatus           string
	SponsorPoolMicrocredits int64
	ModerationState         string
}

func (e *Error) Error() string {
	msg := e.Message
	if (e.Status == 429 || e.Status == 503) && e.RetryAfter > 0 {
		msg += fmt.Sprintf(" (retry in %ds)", e.RetryAfter)
	}
	if e.Code != "" {
		return fmt.Sprintf("%d %s: %s", e.Status, e.Code, msg)
	}
	return fmt.Sprintf("%d: %s", e.Status, msg)
}

// check turns a non-2xx response into *Error.
func check(resp *http.Response, body []byte) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var b struct {
		Error   string `json:"error"`
		Code    any    `json:"code"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
		Reason  string `json:"reason"`

		Missing                 []string `json:"missing"`
		Cap                     *int     `json:"cap"`
		MonetizingAvatars       *int     `json:"monetizingAvatars"`
		PayoutsStatus           string   `json:"payoutsStatus"`
		SponsorPoolMicrocredits int64    `json:"sponsorPoolMicrocredits"`
		ModerationState         string   `json:"moderationState"`
	}
	_ = json.Unmarshal(body, &b)
	e := &Error{Status: resp.StatusCode, Code: b.Error, Message: b.Message, Reason: b.Reason,
		Missing: b.Missing, Cap: b.Cap, MonetizingAvatars: b.MonetizingAvatars, PayoutsStatus: b.PayoutsStatus,
		SponsorPoolMicrocredits: b.SponsorPoolMicrocredits, ModerationState: b.ModerationState}
	if e.Code == "" {
		if s, ok := b.Code.(string); ok {
			e.Code = s
		}
	}
	if e.Message == "" {
		e.Message = b.Detail
	}
	// The OpenAPI validator appends the schema and value; the first line is
	// the useful part.
	if i := strings.Index(e.Message, "\nSchema:"); i >= 0 {
		e.Message = strings.TrimSpace(e.Message[:i])
	}
	if e.Message == "" {
		e.Message = strings.TrimSpace(string(body))
	}
	if e.Status == 401 && e.Message == "" {
		e.Message = "missing or invalid API key or sign-in"
	}
	if e.Status == 429 && e.Code == "" {
		e.Code = "rate_limited"
	}
	if e.Status == 409 || e.Status == 429 || e.Status == 503 {
		// A full or cold pool (503) says when to come back, like a capture limit.
		e.RetryAfter, _ = strconv.Atoi(resp.Header.Get("Retry-After"))
	}
	return e
}

// Quota is the account's capture room from X-RateLimit-* headers: Limit
// captures may be queued or running at once, and Remaining more fit now.
type Quota struct {
	Limit     int `json:"limit"`
	Remaining int `json:"remaining"`
}

func quotaOf(h http.Header) *Quota {
	if h.Get("X-RateLimit-Limit") == "" {
		return nil
	}
	q := &Quota{}
	q.Limit, _ = strconv.Atoi(h.Get("X-RateLimit-Limit"))
	q.Remaining, _ = strconv.Atoi(h.Get("X-RateLimit-Remaining"))
	return q
}

// sentOnce marks a transport failure of a GPU-starting call that carries no
// key: the request may have arrived, so a blind resend could start the work
// twice. The CLI does not resend; the error says what to check first.
func sentOnce(err error, check string) error {
	var apiErr *Error
	if err == nil || errors.As(err, &apiErr) {
		return err
	}
	return fmt.Errorf("%w (it may have reached the server; %s before retrying)", err, check)
}

// idempotent runs a paying call with one Idempotency-Key.
// It waits out a first attempt the server reports still running (409
// in_progress). It does not re-send after a network failure, where it cannot
// tell whether the first attempt arrived; the error carries the key instead,
// so a deliberate retry can reuse it and get the first result back.
func idempotent[T any](c *Client, key string, call func(key string) (T, error)) (T, error) {
	if key == "" {
		key = uuid.NewString()
	}
	waits := 0
	for {
		out, err := call(key)
		if err == nil {
			return out, nil
		}
		var apiErr *Error
		switch {
		case errors.As(err, &apiErr):
			if apiErr.Code != "idempotency_key_in_progress" {
				return out, err
			}
			if waits >= inProgressWaits {
				return out, fmt.Errorf("%w (still running; reuse Idempotency-Key %s to get its result)", err, key)
			}
			waits++
			c.Sleep(c.InProgressSleep)
		default: // transport error: the request may or may not have arrived
			return out, fmt.Errorf("%w (it may have reached the server; check before retrying, and reuse Idempotency-Key %s)", err, key)
		}
	}
}
