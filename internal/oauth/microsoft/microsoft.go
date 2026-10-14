package microsoft

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
)

const (
	loginHost        = "https://login.microsoftonline.com"
	consumerTenantID = "9188040d-6c67-4c5b-b112-36a304b66dad"
	maxTenantKeySets = 64
)

type Identity struct {
	OAuthID       string
	Email         string
	EmailVerified bool
}

type Config struct {
	ClientID   string
	TenantID   string
	HTTPClient *http.Client
	LoginHost  string
}

type Client struct {
	ClientID   string
	TenantID   string
	httpClient *http.Client
	loginHost  string
	keySetsMu  sync.Mutex
	keySets    map[string]*oidc.RemoteKeySet
	keySetFIFO []string
}

func New(cfg Config) (*Client, error) {
	clientID := strings.TrimSpace(cfg.ClientID)
	tenantID := strings.ToLower(strings.TrimSpace(cfg.TenantID))
	if clientID == "" || tenantID == "" {
		return nil, errors.New("microsoft client id and tenant id are required")
	}
	if !validConfiguredTenant(tenantID) {
		return nil, errors.New("microsoft tenant id must be a UUID, common, organizations, or consumers")
	}
	host := strings.TrimRight(cfg.LoginHost, "/")
	if host == "" {
		host = loginHost
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	client = newJWKSHTTPClient(client)
	return &Client{
		ClientID: clientID, TenantID: tenantID, httpClient: client, loginHost: host,
		keySets: make(map[string]*oidc.RemoteKeySet),
	}, nil
}

func (c *Client) Authority() string {
	return c.loginHost + "/" + c.TenantID
}

func (c *Client) VerifyIDToken(ctx context.Context, rawToken, expectedNonce string) (*Identity, error) {
	rawToken = strings.TrimSpace(rawToken)
	expectedNonce = strings.TrimSpace(expectedNonce)
	if rawToken == "" || expectedNonce == "" {
		return nil, errors.New("microsoft id token and nonce are required")
	}

	var unverified struct {
		TenantID string `json:"tid"`
	}
	if err := decodeUnverifiedClaims(rawToken, &unverified); err != nil {
		return nil, fmt.Errorf("decode microsoft id token: %w", err)
	}
	tenantID := strings.ToLower(strings.TrimSpace(unverified.TenantID))
	if !canonicalUUID(tenantID) || !c.allowsTenant(tenantID) {
		return nil, errors.New("microsoft id token tenant is not allowed")
	}

	issuer := c.loginHost + "/" + tenantID + "/v2.0"
	keys := c.keySet(tenantID)
	verifier := oidc.NewVerifier(issuer, keys, &oidc.Config{
		ClientID:             c.ClientID,
		SupportedSigningAlgs: []string{oidc.RS256},
	})
	verified, err := verifier.Verify(oidc.ClientContext(ctx, c.httpClient), rawToken)
	if err != nil {
		return nil, fmt.Errorf("invalid microsoft id token: %w", err)
	}

	var claims struct {
		TenantID             string       `json:"tid"`
		ObjectID             string       `json:"oid"`
		Nonce                string       `json:"nonce"`
		Version              string       `json:"ver"`
		Email                string       `json:"email"`
		PreferredUsername    string       `json:"preferred_username"`
		VerifiedPrimaryEmail string       `json:"verified_primary_email"`
		EmailDomainVerified  flexibleBool `json:"xms_edov"`
	}
	if err := verified.Claims(&claims); err != nil {
		return nil, fmt.Errorf("decode microsoft id token claims: %w", err)
	}
	if claims.Version != "2.0" || subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(expectedNonce)) != 1 {
		return nil, errors.New("microsoft id token nonce or version mismatch")
	}
	objectID := strings.ToLower(strings.TrimSpace(claims.ObjectID))
	if !canonicalUUID(objectID) {
		return nil, errors.New("microsoft id token missing valid object id")
	}
	email := strings.ToLower(strings.TrimSpace(claims.VerifiedPrimaryEmail))
	emailVerified := email != ""
	if email == "" {
		email = strings.ToLower(strings.TrimSpace(claims.Email))
		emailVerified = email != "" && bool(claims.EmailDomainVerified)
	}
	if email == "" {
		email = strings.ToLower(strings.TrimSpace(claims.PreferredUsername))
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Name != "" || !strings.EqualFold(address.Address, email) {
		return nil, errors.New("microsoft id token missing usable email address")
	}
	return &Identity{
		OAuthID:       tenantID + ":" + objectID,
		Email:         email,
		EmailVerified: emailVerified,
	}, nil
}

func (c *Client) keySet(tenantID string) *oidc.RemoteKeySet {
	c.keySetsMu.Lock()
	defer c.keySetsMu.Unlock()
	if keys := c.keySets[tenantID]; keys != nil {
		return keys
	}
	if len(c.keySetFIFO) >= maxTenantKeySets {
		delete(c.keySets, c.keySetFIFO[0])
		c.keySetFIFO = c.keySetFIFO[1:]
	}
	keys := oidc.NewRemoteKeySet(
		oidc.ClientContext(context.Background(), c.httpClient),
		c.loginHost+"/"+tenantID+"/discovery/v2.0/keys",
	)
	c.keySets[tenantID] = keys
	c.keySetFIFO = append(c.keySetFIFO, tenantID)
	return keys
}

func (c *Client) allowsTenant(tenantID string) bool {
	switch c.TenantID {
	case "common":
		return true
	case "organizations":
		return tenantID != consumerTenantID
	case "consumers":
		return tenantID == consumerTenantID
	default:
		return tenantID == c.TenantID
	}
}

func validConfiguredTenant(tenantID string) bool {
	switch tenantID {
	case "common", "organizations", "consumers":
		return true
	default:
		return canonicalUUID(tenantID)
	}
}

func canonicalUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && strings.EqualFold(parsed.String(), value)
}

func decodeUnverifiedClaims(rawToken string, destination any) error {
	parts := strings.Split(rawToken, ".")
	if len(parts) != 3 {
		return errors.New("malformed JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, destination)
}

type flexibleBool bool

func (b *flexibleBool) UnmarshalJSON(data []byte) error {
	var value bool
	if err := json.Unmarshal(data, &value); err == nil {
		*b = flexibleBool(value)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return errors.New("microsoft boolean claim has invalid type")
	}
	switch strings.ToLower(text) {
	case "true":
		*b = true
	case "false", "":
		*b = false
	default:
		return errors.New("microsoft boolean claim has invalid value")
	}
	return nil
}
