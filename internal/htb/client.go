// Package htb is a client for Hack The Box's internal, unofficial v4 REST API.
//
// HTB publishes no API for regular accounts, so this package talks to the same
// endpoints the HTB web app uses, authenticated with a user-generated App
// Token. That API is undocumented and may change without notice, so response
// parsing is deliberately strict: an unexpected shape is surfaced as
// ErrUnexpectedResponse instead of being mapped to zero values (NFR-8).
package htb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultBaseURL is the API root used by HTB's web app for machines and labs.
const DefaultBaseURL = "https://labs.hackthebox.com/api/v4"

// DefaultAccountURL is the API root htb-cli uses for connection status.
// Labs and the main site have both served this endpoint; the client tries
// DefaultBaseURL first and falls back here on 404.
const DefaultAccountURL = "https://www.hackthebox.com/api/v4"

// DefaultProfileTTL is how long a fetched machine profile is reused before it is
// requested again.
//
// A profile carries OS, difficulty, avatar and the user/root flags. Those change
// at most a couple of times per machine session, so reusing one keeps a 30s poll
// from spending two requests on every tick. The trade-off is that the flag
// markers can lag by up to this long.
const DefaultProfileTTL = 10 * time.Minute

// maxResponseBytes caps how much of a response is read, guarding against a
// misbehaving or unexpected server.
const maxResponseBytes = 1 << 20 // 1 MiB

// Sentinel errors callers can match with errors.Is.
var (
	// ErrAuth indicates the App Token was rejected or has expired.
	ErrAuth = errors.New("htb: authentication failed")

	// ErrRateLimited indicates HTB throttled the request.
	ErrRateLimited = errors.New("htb: rate limited")

	// ErrUnexpectedResponse indicates the API returned a shape this client does
	// not recognize, for example after an upstream change.
	ErrUnexpectedResponse = errors.New("htb: unexpected API response")

	// ErrNotFound indicates the endpoint does not exist. Callers that probe
	// optional activity endpoints treat this as "nothing active" rather than
	// a hard failure. A 404 on a required endpoint is still returned.
	ErrNotFound = errors.New("htb: not found")
)

// RateLimitError reports a 429 response, including any Retry-After hint.
type RateLimitError struct {
	RetryAfter time.Duration
}

// Error implements error.
func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("htb: rate limited, retry after %s", e.RetryAfter)
	}
	return "htb: rate limited"
}

// Is lets errors.Is(err, ErrRateLimited) match.
func (e *RateLimitError) Is(target error) bool { return target == ErrRateLimited }

// Machine describes a single HTB machine.
type Machine struct {
	ID         int
	Name       string
	OS         string
	Difficulty string
	IP         string
	AvatarURL  string
	ExpiresAt  time.Time
	// UserOwned and RootOwned report whether the authenticated user has
	// submitted the user and root flags. They come from the machine profile.
	UserOwned bool
	RootOwned bool
}

// Challenge is a spawned HTB challenge instance.
type Challenge struct {
	ID         int
	Name       string
	Category   string
	Difficulty string
	AvatarURL  string
	ExpiresAt  time.Time
}

// VPN is the authenticated user's Hack The Box VPN connection.
// The assigned tunnel address is intentionally not retained.
type VPN struct {
	Connected bool
	// Product is the HTB product the tunnel is attached to, when the API
	// reports one (for example "labs" or "fortresses").
	Product string
}

// User describes the authenticated user's standing on HTB.
type User struct {
	ID     int
	Name   string
	Rank   string
	Points int
}

// Activity is the user's current HTB activity. A nil Machine and a nil
// Challenge mean nothing is spawned. A nil User means rank and points were
// not loaded. VPN is filled in by the scheduler when it asks for connection
// status; CurrentActivity leaves it zero.
type Activity struct {
	Machine   *Machine
	Challenge *Challenge
	User      *User
	VPN       VPN
}

// Fetcher fetches HTB state. It exists so the mapping and scheduling layers can
// be tested without a live HTB account.
type Fetcher interface {
	// CurrentActivity reports the active machine or challenge, if any.
	CurrentActivity(ctx context.Context) (*Activity, error)
	// User reports the authenticated user's rank and points.
	User(ctx context.Context) (*User, error)
	// VPNConnected reports whether an HTB VPN tunnel is up.
	VPNConnected(ctx context.Context) (VPN, error)
}

// Client is an HTB API client.
//
// It is safe for concurrent use. The only state it keeps is a single-entry
// machine profile cache.
type Client struct {
	baseURL    string
	accountURL string
	token      string
	httpClient *http.Client

	// profileTTL is how long a fetched machine profile stays usable.
	// Zero disables the cache.
	profileTTL time.Duration

	// profileMu guards profile.
	profileMu sync.Mutex
	profile   *cachedProfile

	// now supplies the current time for cache expiry; nil means time.Now.
	now func() time.Time
}

// cachedProfile is the last machine profile fetched, keyed by machine id.
type cachedProfile struct {
	id        int
	profile   machineProfile
	fetchedAt time.Time
}

// Option customizes a Client.
type Option func(*Client)

// WithBaseURL overrides the API root, for example to point at a test server.
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithAccountURL overrides the host used when connection status is not served
// from the labs API root.
func WithAccountURL(url string) Option {
	return func(c *Client) { c.accountURL = url }
}

// WithHTTPClient overrides the underlying HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// WithProfileTTL overrides how long a fetched machine profile is reused. Zero
// disables the cache, so every poll fetches the profile again.
func WithProfileTTL(ttl time.Duration) Option {
	return func(c *Client) { c.profileTTL = ttl }
}

// NewClient returns an HTB client authenticating with the given App Token.
func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		baseURL:    DefaultBaseURL,
		accountURL: DefaultAccountURL,
		token:      token,
		profileTTL: DefaultProfileTTL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
			// Do not follow redirects: the API signals an expired or invalid
			// token with a 302 to /login, which must be observed here.
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// clock returns the current time, honoring the test hook in now.
func (c *Client) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// CurrentActivity reports what the user is currently doing on HTB.
//
// A lab machine wins over a season machine, which wins over a spawned
// challenge. No spawned target yields an Activity with nil Machine and
// Challenge, not an error. VPN status is not fetched here.
func (c *Client) CurrentActivity(ctx context.Context) (*Activity, error) {
	machine, err := c.activeMachine(ctx)
	if err != nil {
		return nil, err
	}
	if machine != nil {
		return &Activity{Machine: machine}, nil
	}

	machine, err = c.activeSeasonMachine(ctx)
	if err != nil {
		return nil, err
	}
	if machine != nil {
		return &Activity{Machine: machine}, nil
	}

	challenge, err := c.activeChallenge(ctx)
	if err != nil {
		return nil, err
	}
	return &Activity{Challenge: challenge}, nil
}

// User reports the authenticated user's name, rank and points.
//
// It resolves the account id from /user/info and then reads rank and points
// from /user/profile/basic/{id}, mirroring the calls HTB's web app makes.
func (c *Client) User(ctx context.Context) (*User, error) {
	var info struct {
		Info *struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"info"`
	}
	if err := c.get(ctx, "/user/info", &info); err != nil {
		return nil, err
	}
	if info.Info == nil || info.Info.ID == 0 {
		return nil, fmt.Errorf("%w: /user/info is missing an id", ErrUnexpectedResponse)
	}

	var profile struct {
		Profile *struct {
			Rank   string `json:"rank"`
			Points int    `json:"points"`
		} `json:"profile"`
	}
	if err := c.get(ctx, fmt.Sprintf("/user/profile/basic/%d", info.Info.ID), &profile); err != nil {
		return nil, err
	}
	if profile.Profile == nil {
		return nil, fmt.Errorf("%w: /user/profile/basic/%d is missing a profile", ErrUnexpectedResponse, info.Info.ID)
	}

	return &User{
		ID:     info.Info.ID,
		Name:   info.Info.Name,
		Rank:   profile.Profile.Rank,
		Points: profile.Profile.Points,
	}, nil
}

// machineProfile holds the subset of /machine/profile/{id} this app cares about.
type machineProfile struct {
	Name       string `json:"name"`
	OS         string `json:"os"`
	Difficulty string `json:"difficultyText"`
	Avatar     string `json:"avatar"`
	UserOwned  bool   `json:"authUserInUserOwns"`
	RootOwned  bool   `json:"authUserInRootOwns"`
}

// applyProfile copies display fields from a machine profile onto m.
func applyProfile(m *Machine, p *machineProfile) {
	m.Name = p.Name
	m.OS = p.OS
	m.Difficulty = p.Difficulty
	m.AvatarURL = resolveAssetURL(p.Avatar)
	m.UserOwned = p.UserOwned
	m.RootOwned = p.RootOwned
}

// machineProfile fetches display details for the given machine id.
func (c *Client) machineProfile(ctx context.Context, id int) (*machineProfile, error) {
	var resp struct {
		Info *machineProfile `json:"info"`
	}
	if err := c.get(ctx, fmt.Sprintf("/machine/profile/%d", id), &resp); err != nil {
		return nil, err
	}
	if resp.Info == nil || resp.Info.Name == "" {
		return nil, fmt.Errorf("%w: /machine/profile/%d is missing a name", ErrUnexpectedResponse, id)
	}
	return resp.Info, nil
}

// get performs an authenticated GET and decodes the JSON body into out.
func (c *Client) get(ctx context.Context, path string, out any) error {
	return c.decode(ctx, c.baseURL+path, path, out)
}

// getOptional is get, except a 404 yields ok=false and a nil error.
func (c *Client) getOptional(ctx context.Context, path string, out any) (bool, error) {
	err := c.get(ctx, path, out)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// decode GETs rawURL and decodes the JSON body into out. label is used in errors.
func (c *Client) decode(ctx context.Context, rawURL, label string, out any) error {
	body, err := c.getBytes(ctx, rawURL, label)
	if err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: decoding htb %s: %v", ErrUnexpectedResponse, label, err)
	}
	return nil
}

// getBytes performs an authenticated GET and returns the response body.
func (c *Client) getBytes(ctx context.Context, rawURL, label string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", "htb-presence")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling htb %s: %w", label, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return nil, &RateLimitError{RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	case resp.StatusCode == http.StatusFound && strings.Contains(resp.Header.Get("Location"), "/login"):
		return nil, fmt.Errorf("%w: token rejected (redirected to login)", ErrAuth)
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return nil, fmt.Errorf("%w: status %d", ErrAuth, resp.StatusCode)
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("%w: %s", ErrNotFound, label)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("htb %s: unexpected status %d", label, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("reading htb %s response: %w", label, err)
	}
	return body, nil
}

// parseRetryAfter interprets a Retry-After header, which may be either a number
// of seconds or an HTTP date. It returns 0 when no usable hint is present.
func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if secs, err := strconv.Atoi(value); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(value); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// parseHTBTime parses the timestamp formats the internal API has been observed
// to use.
func parseHTBTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time %q", value)
}

// resolveAssetURL turns an API-relative asset path into an absolute URL, leaving
// absolute URLs untouched. The API has returned both forms over time.
func resolveAssetURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	return "https://" + hostFromURL(DefaultBaseURL) + "/" + strings.TrimLeft(raw, "/")
}

// hostFromURL returns the host part of an absolute URL, or the input unchanged
// when it cannot be parsed.
func hostFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	return u.Host
}
