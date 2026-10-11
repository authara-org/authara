package google

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"google.golang.org/api/idtoken"
)

const tokenEndpoint = "https://oauth2.googleapis.com/token"

type Identity struct {
	OAuthID       string
	Email         string
	EmailVerified bool
}

type Client struct {
	ClientID      string
	ClientSecret  string
	HTTPClient    *http.Client
	TokenEndpoint string
	verifyToken   func(context.Context, string, string) (*idtoken.Payload, error)
}

func New(clientID string, clientSecret ...string) *Client {
	secret := ""
	if len(clientSecret) > 0 {
		secret = clientSecret[0]
	}
	return &Client{
		ClientID:      clientID,
		ClientSecret:  secret,
		HTTPClient:    http.DefaultClient,
		TokenEndpoint: tokenEndpoint,
		verifyToken:   idtoken.Validate,
	}
}

func (c *Client) VerifyIDToken(ctx context.Context, rawIDToken string, expectedNonce string) (*Identity, error) {
	if rawIDToken == "" {
		return nil, errors.New("google id token is empty")
	}

	payload, err := c.verifyToken(ctx, rawIDToken, c.ClientID)
	if err != nil {
		return nil, fmt.Errorf("invalid google id token: %w", err)
	}

	sub, ok := payload.Claims["sub"].(string)
	if !ok || sub == "" {
		return nil, errors.New("google id token missing subject (sub)")
	}

	email, _ := payload.Claims["email"].(string)
	if email == "" {
		return nil, errors.New("google id token missing email")
	}

	emailVerified, _ := payload.Claims["email_verified"].(bool)
	if !emailVerified {
		return nil, errors.New("google id token email is not verified")
	}

	if expectedNonce != "" {
		nonce, _ := payload.Claims["nonce"].(string)
		if nonce == "" || nonce != expectedNonce {
			return nil, errors.New("google id token nonce mismatch")
		}
	}

	return &Identity{
		OAuthID:       sub,
		Email:         email,
		EmailVerified: emailVerified,
	}, nil
}

func (c *Client) ExchangeAuthorizationCode(ctx context.Context, code string, redirectURI string) (*Identity, error) {
	code = strings.TrimSpace(code)
	redirectURI = strings.TrimSpace(redirectURI)
	if code == "" {
		return nil, errors.New("google authorization code is empty")
	}
	if strings.TrimSpace(c.ClientSecret) == "" {
		return nil, errors.New("google client secret is not configured")
	}
	if err := validatePopupRedirectURI(redirectURI); err != nil {
		return nil, err
	}

	form := url.Values{
		"client_id":     {c.ClientID},
		"client_secret": {c.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create google token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchange google authorization code: %w", err)
	}
	defer res.Body.Close()

	var payload struct {
		IDToken          string `json:"id_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode google token response: %w", err)
	}
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusMultipleChoices {
		if payload.Error == "" {
			payload.Error = res.Status
		}
		return nil, fmt.Errorf("google token exchange rejected: %s", payload.Error)
	}
	if payload.IDToken == "" {
		return nil, errors.New("google token response missing id_token")
	}
	return c.VerifyIDToken(ctx, payload.IDToken, "")
}

func validatePopupRedirectURI(raw string) error {
	u, err := url.ParseRequestURI(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("google redirect_uri must be an HTTP origin")
	}
	if u.Scheme == "http" && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" && u.Hostname() != "::1" {
		return errors.New("google redirect_uri must use HTTPS outside localhost")
	}
	return nil
}
