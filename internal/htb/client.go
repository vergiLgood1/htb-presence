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
	"time"
)

// DefaultBaseURL is the API root used by HTB's web app for machines and labs.
const DefaultBaseURL = "https://labs.hackthebox.com/api/v4"

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
}

// User describes the authenticated user's standing on HTB.
type User struct {
	Name   string
	Rank   string
	Points int
}

// Activity is the user's current HTB activity. A nil Machine means the user has
// no active machine; a nil User means rank/points were not loaded.
type Activity struct {
	Machine *Machine
	User    *User
}

// Fetcher fetches HTB state. It exists so the mapping and scheduling layers can
// be tested without a live HTB account.
type Fetcher interface {
	// CurrentActivity reports the active machine, if any.
	CurrentActivity(ctx context.Context) (*Activity, error)
	// User reports the authenticated user's rank and points.
	User(ctx context.Context) (*User, error)
}

// Client is an HTB API client.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// Option customizes a Client.
type Option func(*Client)

// WithBaseURL overrides the API root, for example to point at a test server.
func WithBaseURL(url string) Option {
	return func(c *Client) { c.baseURL = url }
}

// WithHTTPClient overrides the underlying HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) { c.httpClient = hc }
}

// NewClient returns an HTB client authenticating with the given App Token.
func NewClient(token string, opts ...Option) *Client {
	c := &Client{
		baseURL: DefaultBaseURL,
		token:   token,
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

// CurrentActivity reports what the user is currently doing on HTB.
//
// A user with no active machine yields an Activity with a nil Machine, not an
// error.
func (c *Client) CurrentActivity(ctx context.Context) (*Activity, error) {
	var active struct {
		Info *struct {
			ID        int    `json:"id"`
			IP        string `json:"ip"`
			ExpiresAt string `json:"expires_at"`
		} `json:"info"`
	}
	if err := c.get(ctx, "/machine/active", &active); err != nil {
		return nil, err
	}
	if active.Info == nil {
		return &Activity{}, nil
	}
	if active.Info.ID == 0 {
		return nil, fmt.Errorf("%w: /machine/active returned info without an id", ErrUnexpectedResponse)
	}

	machine := &Machine{
		ID: active.Info.ID,
		IP: active.Info.IP,
	}
	if active.Info.ExpiresAt != "" {
		if t, err := parseHTBTime(active.Info.ExpiresAt); err == nil {
			machine.ExpiresAt = t
		}
	}

	profile, err := c.machineProfile(ctx, machine.ID)
	if err != nil {
		return nil, err
	}
	machine.Name = profile.Name
	machine.OS = profile.OS
	machine.Difficulty = profile.Difficulty
	machine.AvatarURL = resolveAssetURL(profile.Avatar)

	return &Activity{Machine: machine}, nil
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("User-Agent", "htb-presence")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling htb %s: %w", path, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return &RateLimitError{RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After"))}
	case resp.StatusCode == http.StatusFound && strings.Contains(resp.Header.Get("Location"), "/login"):
		return fmt.Errorf("%w: token rejected (redirected to login)", ErrAuth)
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("%w: status %d", ErrAuth, resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("htb %s: unexpected status %d", path, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("reading htb %s response: %w", path, err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("%w: decoding htb %s: %v", ErrUnexpectedResponse, path, err)
	}
	return nil
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
