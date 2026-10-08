package apple

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
)

const (
	Issuer    = "https://appleid.apple.com"
	JWKSURL   = "https://appleid.apple.com/auth/keys"
	TokenURL  = "https://appleid.apple.com/auth/token"
	RevokeURL = "https://appleid.apple.com/auth/revoke"
)

const maxResponseBytes = 1 << 20

type Identity struct {
	OAuthID       string
	Email         string
	EmailVerified bool
	PrivateRelay  bool
}

type ExchangeResult struct {
	Identity     Identity
	RefreshToken string
}

type Config struct {
	ClientID    string
	TeamID      string
	KeyID       string
	PrivateKey  *ecdsa.PrivateKey
	RedirectURI string
	HTTPClient  *http.Client
	Issuer      string
	JWKSURL     string
	TokenURL    string
	RevokeURL   string
	Now         func() time.Time
}

type Client struct {
	clientID    string
	teamID      string
	keyID       string
	privateKey  *ecdsa.PrivateKey
	redirectURI string
	httpClient  *http.Client
	issuer      string
	tokenURL    string
	revokeURL   string
	verifier    *oidc.IDTokenVerifier
	now         func() time.Time
}

func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.TeamID) == "" || strings.TrimSpace(cfg.KeyID) == "" {
		return nil, errors.New("apple client id, team id, and key id are required")
	}
	if cfg.PrivateKey == nil {
		return nil, errors.New("apple private key is required")
	}
	if strings.TrimSpace(cfg.RedirectURI) == "" {
		return nil, errors.New("apple redirect URI is required")
	}
	clientID := strings.TrimSpace(cfg.ClientID)
	teamID := strings.TrimSpace(cfg.TeamID)
	keyID := strings.TrimSpace(cfg.KeyID)
	redirectURI := strings.TrimSpace(cfg.RedirectURI)
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	issuer := cfg.Issuer
	if issuer == "" {
		issuer = Issuer
	}
	jwksURL := cfg.JWKSURL
	if jwksURL == "" {
		jwksURL = JWKSURL
	}
	tokenURL := cfg.TokenURL
	if tokenURL == "" {
		tokenURL = TokenURL
	}
	revokeURL := cfg.RevokeURL
	if revokeURL == "" {
		revokeURL = RevokeURL
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	keyContext := oidc.ClientContext(context.Background(), client)
	keys := oidc.NewRemoteKeySet(keyContext, jwksURL)
	return &Client{
		clientID: clientID, teamID: teamID, keyID: keyID,
		privateKey: cfg.PrivateKey, redirectURI: redirectURI, httpClient: client,
		issuer: issuer, tokenURL: tokenURL, revokeURL: revokeURL, now: now,
		verifier: oidc.NewVerifier(issuer, keys, &oidc.Config{
			ClientID:             clientID,
			SupportedSigningAlgs: []string{oidc.RS256},
		}),
	}, nil
}

func (c *Client) Exchange(ctx context.Context, code, expectedNonce string) (ExchangeResult, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return ExchangeResult{}, errors.New("apple authorization code is empty")
	}
	if expectedNonce == "" {
		return ExchangeResult{}, errors.New("apple nonce is empty")
	}
	clientSecret, err := c.clientSecret()
	if err != nil {
		return ExchangeResult{}, err
	}
	form := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {clientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {c.redirectURI},
	}
	var response tokenResponse
	if err := c.postForm(ctx, c.tokenURL, form, &response); err != nil {
		return ExchangeResult{}, fmt.Errorf("exchange apple authorization code: %w", err)
	}
	if response.IDToken == "" || response.RefreshToken == "" {
		return ExchangeResult{}, errors.New("apple token response is incomplete")
	}
	identity, err := c.verifyIDToken(ctx, response.IDToken, expectedNonce)
	if err != nil {
		return ExchangeResult{}, err
	}
	return ExchangeResult{Identity: identity, RefreshToken: response.RefreshToken}, nil
}

func (c *Client) Revoke(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil
	}
	clientSecret, err := c.clientSecret()
	if err != nil {
		return err
	}
	form := url.Values{
		"client_id":       {c.clientID},
		"client_secret":   {clientSecret},
		"token":           {refreshToken},
		"token_type_hint": {"refresh_token"},
	}
	if err := c.postForm(ctx, c.revokeURL, form, nil); err != nil {
		return fmt.Errorf("revoke apple refresh token: %w", err)
	}
	return nil
}

func (c *Client) verifyIDToken(ctx context.Context, rawToken, expectedNonce string) (Identity, error) {
	verified, err := c.verifier.Verify(oidc.ClientContext(ctx, c.httpClient), rawToken)
	if err != nil {
		return Identity{}, fmt.Errorf("invalid apple id token: %w", err)
	}
	var claims identityClaims
	if err := verified.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("decode apple id token claims: %w", err)
	}
	if claims.Subject == "" {
		return Identity{}, errors.New("apple id token missing subject")
	}
	if claims.Nonce == "" || claims.Nonce != expectedNonce {
		return Identity{}, errors.New("apple id token nonce mismatch")
	}
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	return Identity{
		OAuthID: claims.Subject, Email: email,
		EmailVerified: bool(claims.EmailVerified), PrivateRelay: bool(claims.IsPrivateEmail),
	}, nil
}

func (c *Client) clientSecret() (string, error) {
	now := c.now().UTC()
	claims := jwt.RegisteredClaims{
		Issuer: c.teamID, Subject: c.clientID,
		Audience: jwt.ClaimStrings{Issuer},
		IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = c.keyID
	signed, err := token.SignedString(c.privateKey)
	if err != nil {
		return "", fmt.Errorf("sign apple client secret: %w", err)
	}
	return signed, nil
}

func (c *Client) postForm(ctx context.Context, endpoint string, form url.Values, destination any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return err
	}
	if len(body) > maxResponseBytes {
		return errors.New("apple response exceeds size limit")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		var appleError errorResponse
		_ = json.Unmarshal(body, &appleError)
		if appleError.Error != "" {
			return fmt.Errorf("apple returned %s", appleError.Error)
		}
		return fmt.Errorf("apple returned HTTP %d", response.StatusCode)
	}
	if destination == nil || len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, destination); err != nil {
		return fmt.Errorf("decode apple response: %w", err)
	}
	return nil
}

type tokenResponse struct {
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type identityClaims struct {
	Subject        string       `json:"sub"`
	Nonce          string       `json:"nonce"`
	Email          string       `json:"email"`
	EmailVerified  flexibleBool `json:"email_verified"`
	IsPrivateEmail flexibleBool `json:"is_private_email"`
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
		return errors.New("apple boolean claim has invalid type")
	}
	switch strings.ToLower(text) {
	case "true":
		*b = true
	case "false", "":
		*b = false
	default:
		return errors.New("apple boolean claim has invalid value")
	}
	return nil
}
