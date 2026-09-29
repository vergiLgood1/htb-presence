package htb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// activeMachine loads /machine/active. A nil machine means nothing is spawned.
func (c *Client) activeMachine(ctx context.Context) (*Machine, error) {
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
		return nil, nil
	}
	if active.Info.ID == 0 {
		return nil, fmt.Errorf("%w: /machine/active returned info without an id", ErrUnexpectedResponse)
	}

	machine := &Machine{ID: active.Info.ID, IP: active.Info.IP, ExpiresAt: parseOptionalTime(active.Info.ExpiresAt)}
	if err := c.fillMachine(ctx, machine); err != nil {
		return nil, err
	}
	return machine, nil
}

// activeSeasonMachine loads /season/machine/active.
//
// The endpoint describes the season's current machine whether or not the user
// is on it, so an unspawned entry is not activity: the machine counts only when
// play_info reports a spawned or active instance, or when the API assigned an
// instance IP. A 404, a null data object, or an unspawned machine means there
// is no season machine.
func (c *Client) activeSeasonMachine(ctx context.Context) (*Machine, error) {
	var resp struct {
		Data *struct {
			ID        int    `json:"id"`
			IP        string `json:"ip"`
			ExpiresAt string `json:"expires_at"`
			PlayInfo  *struct {
				IsSpawned bool   `json:"is_spawned"`
				IsActive  bool   `json:"is_active"`
				ExpiresAt string `json:"expires_at"`
			} `json:"play_info"`
		} `json:"data"`
	}
	ok, err := c.getOptional(ctx, "/season/machine/active", &resp)
	if err != nil || !ok || resp.Data == nil {
		return nil, err
	}
	if resp.Data.ID == 0 {
		return nil, fmt.Errorf("%w: /season/machine/active returned data without an id", ErrUnexpectedResponse)
	}

	// Spawn evidence lives in play_info, not in the presence of a data
	// object. is_spawning alone does not count: the instance cannot be
	// touched yet, so naming it in the presence would be premature.
	spawned := strings.TrimSpace(resp.Data.IP) != ""
	if resp.Data.PlayInfo != nil {
		spawned = spawned || resp.Data.PlayInfo.IsSpawned || resp.Data.PlayInfo.IsActive
	}
	if !spawned {
		return nil, nil
	}

	expires := resp.Data.ExpiresAt
	if resp.Data.PlayInfo != nil && resp.Data.PlayInfo.ExpiresAt != "" {
		expires = resp.Data.PlayInfo.ExpiresAt
	}
	machine := &Machine{ID: resp.Data.ID, IP: resp.Data.IP, ExpiresAt: parseOptionalTime(expires)}
	if err := c.fillMachine(ctx, machine); err != nil {
		return nil, err
	}
	return machine, nil
}

// activeChallenge loads /challenge/active. A 404 means this deployment has no
// such endpoint and is treated as "no challenge", so machine presence keeps
// working. A 200 with an unexpected shape is still an error.
func (c *Client) activeChallenge(ctx context.Context) (*Challenge, error) {
	var resp struct {
		Info *challengeFields `json:"info"`
	}
	ok, err := c.getOptional(ctx, "/challenge/active", &resp)
	if err != nil || !ok || resp.Info == nil {
		return nil, err
	}
	if resp.Info.ID == 0 {
		return nil, fmt.Errorf("%w: /challenge/active returned info without an id", ErrUnexpectedResponse)
	}

	fields := *resp.Info
	if strings.TrimSpace(fields.name()) == "" {
		detail, err := c.challengeInfo(ctx, fields.ID)
		if err != nil {
			return nil, err
		}
		fields = mergeChallenge(detail, fields)
	}
	return fields.toChallenge(), nil
}

// challengeInfo loads /challenge/info/{id}.
func (c *Client) challengeInfo(ctx context.Context, id int) (challengeFields, error) {
	var raw map[string]json.RawMessage
	path := fmt.Sprintf("/challenge/info/%d", id)
	if err := c.get(ctx, path, &raw); err != nil {
		return challengeFields{}, err
	}
	for _, key := range []string{"info", "challenge"} {
		blob, ok := raw[key]
		if !ok || bytes.Equal(bytes.TrimSpace(blob), []byte("null")) {
			continue
		}
		var fields challengeFields
		if err := json.Unmarshal(blob, &fields); err != nil {
			return challengeFields{}, fmt.Errorf("%w: decoding %s: %v", ErrUnexpectedResponse, path, err)
		}
		if strings.TrimSpace(fields.name()) != "" {
			return fields, nil
		}
	}
	return challengeFields{}, fmt.Errorf("%w: %s is missing a name", ErrUnexpectedResponse, path)
}

// fillMachine loads /machine/profile/{id} onto m, reusing the profile fetched
// for the same machine while it is still fresh. A poll therefore costs one
// request instead of two for as long as a machine session lasts.
func (c *Client) fillMachine(ctx context.Context, m *Machine) error {
	if profile, ok := c.cachedMachineProfile(m.ID); ok {
		applyProfile(m, &profile)
		return nil
	}

	profile, err := c.machineProfile(ctx, m.ID)
	if err != nil {
		return err
	}
	c.storeMachineProfile(m.ID, *profile)
	applyProfile(m, profile)
	return nil
}

// cachedMachineProfile returns the cached profile for id while it is fresh.
func (c *Client) cachedMachineProfile(id int) (machineProfile, bool) {
	if c.profileTTL <= 0 {
		return machineProfile{}, false
	}

	c.profileMu.Lock()
	defer c.profileMu.Unlock()

	if c.profile == nil || c.profile.id != id {
		return machineProfile{}, false
	}
	if c.clock().Sub(c.profile.fetchedAt) >= c.profileTTL {
		return machineProfile{}, false
	}
	return c.profile.profile, true
}

// storeMachineProfile remembers the profile for id. Only the most recent
// machine is kept, which bounds the cache without needing eviction.
func (c *Client) storeMachineProfile(id int, profile machineProfile) {
	if c.profileTTL <= 0 {
		return
	}

	c.profileMu.Lock()
	defer c.profileMu.Unlock()

	c.profile = &cachedProfile{id: id, profile: profile, fetchedAt: c.clock()}
}

// VPNConnected reports whether the account has an HTB VPN tunnel up.
//
// It tries the labs API root first and, on 404 only, the account API root.
func (c *Client) VPNConnected(ctx context.Context) (VPN, error) {
	const label = "/connection/status"
	body, err := c.getBytes(ctx, c.baseURL+label, label)
	if errors.Is(err, ErrNotFound) && c.accountURL != "" && c.accountURL != c.baseURL {
		body, err = c.getBytes(ctx, c.accountURL+label, label)
	}
	if err != nil {
		return VPN{}, err
	}
	return parseConnectionStatus(body)
}

// challengeFields is the subset of a challenge object this app displays.
type challengeFields struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	CategoryName   string `json:"category_name"`
	Category       string `json:"category"`
	Difficulty     string `json:"difficulty"`
	DifficultyText string `json:"difficultyText"`
	Avatar         string `json:"avatar"`
	ExpiresAt      string `json:"expires_at"`
}

func (f challengeFields) name() string { return f.Name }

func (f challengeFields) category() string {
	if strings.TrimSpace(f.CategoryName) != "" {
		return f.CategoryName
	}
	return f.Category
}

func (f challengeFields) difficulty() string {
	if strings.TrimSpace(f.DifficultyText) != "" {
		return f.DifficultyText
	}
	return f.Difficulty
}

func (f challengeFields) toChallenge() *Challenge {
	return &Challenge{
		ID:         f.ID,
		Name:       f.Name,
		Category:   f.category(),
		Difficulty: f.difficulty(),
		AvatarURL:  resolveAssetURL(f.Avatar),
		ExpiresAt:  parseOptionalTime(f.ExpiresAt),
	}
}

// mergeChallenge fills gaps in overlay with values from base. overlay wins
// when it already has a value, and its expiry is kept when set.
func mergeChallenge(base, overlay challengeFields) challengeFields {
	if overlay.Name == "" {
		overlay.Name = base.Name
	}
	if overlay.category() == "" {
		overlay.CategoryName = base.category()
	}
	if overlay.difficulty() == "" {
		overlay.DifficultyText = base.difficulty()
	}
	if overlay.Avatar == "" {
		overlay.Avatar = base.Avatar
	}
	if overlay.ExpiresAt == "" {
		overlay.ExpiresAt = base.ExpiresAt
	}
	if overlay.ID == 0 {
		overlay.ID = base.ID
	}
	return overlay
}

// parseConnectionStatus accepts the array shape htb-cli reads and the older
// object shape from the community endpoint map.
func parseConnectionStatus(body []byte) (VPN, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return VPN{}, fmt.Errorf("%w: empty connection status", ErrUnexpectedResponse)
	}
	if body[0] == '[' {
		return parseConnectionArray(body)
	}
	return parseConnectionObject(body)
}

func parseConnectionArray(body []byte) (VPN, error) {
	var items []struct {
		Product    string `json:"product"`
		Type       string `json:"type"`
		Connection *struct {
			IP4 string `json:"ip4"`
		} `json:"connection"`
	}
	if err := json.Unmarshal(body, &items); err != nil {
		return VPN{}, fmt.Errorf("%w: decoding connection status: %v", ErrUnexpectedResponse, err)
	}
	if len(items) == 0 {
		return VPN{}, nil
	}
	var products []string
	connected := false
	for _, item := range items {
		if item.Connection == nil {
			continue
		}
		connected = true
		product := strings.TrimSpace(item.Product)
		if product == "" {
			product = strings.TrimSpace(item.Type)
		}
		if product != "" && !contains(products, product) {
			products = append(products, product)
		}
	}
	if !connected {
		return VPN{}, fmt.Errorf("%w: connection status entries have no connection", ErrUnexpectedResponse)
	}
	return VPN{Connected: true, Product: strings.Join(products, ", ")}, nil
}

func parseConnectionObject(body []byte) (VPN, error) {
	var obj struct {
		Status     json.RawMessage `json:"status"`
		Connection json.RawMessage `json:"connection"`
	}
	if err := json.Unmarshal(body, &obj); err != nil {
		return VPN{}, fmt.Errorf("%w: decoding connection status: %v", ErrUnexpectedResponse, err)
	}
	if on, ok := statusConnected(obj.Status); ok {
		return VPN{Connected: on}, nil
	}
	switch classifyConnection(obj.Connection) {
	case connectionOpen:
		return VPN{Connected: true}, nil
	case connectionNotConnected:
		return VPN{Connected: false}, nil
	default:
		return VPN{}, fmt.Errorf("%w: connection status is missing status", ErrUnexpectedResponse)
	}
}

// connectionState classifies the connection field of a connection-status
// object.
type connectionState int

const (
	// connectionUnknown means the field was absent or null, so it says
	// nothing on its own.
	connectionUnknown connectionState = iota
	// connectionOpen means the field was an object holding tunnel addresses.
	connectionOpen
	// connectionNotConnected means the field carried the API's "not
	// connected" marker.
	connectionNotConnected
)

// classifyConnection reads the connection field, which the API has returned as
// an object while a tunnel is up and as the string "not connected" when it is
// not.
func classifyConnection(raw json.RawMessage) connectionState {
	raw = bytes.TrimSpace(raw)
	switch {
	case len(raw) == 0, bytes.Equal(raw, []byte("null")):
		return connectionUnknown
	case raw[0] == '{':
		return connectionOpen
	default:
		return connectionNotConnected
	}
}

// statusConnected reports whether a status value means connected, and whether
// the value was a recognized bool, number, or string.
func statusConnected(raw json.RawMessage) (bool, bool) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return false, false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b, true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "1", "true", "connected":
			return true, true
		default:
			return false, true
		}
	}
	var n float64
	if err := json.Unmarshal(raw, &n); err == nil {
		return n != 0, true
	}
	return false, false
}

func parseOptionalTime(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	t, err := parseHTBTime(value)
	if err != nil {
		return time.Time{}
	}
	return t
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
