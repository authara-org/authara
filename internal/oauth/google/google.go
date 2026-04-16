package google

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/api/idtoken"
)

type Identity struct {
	OAuthID string
	Email   string
}

type Client struct {
	ClientID string
}

func New(clientID string) *Client {
	return &Client{
		ClientID: clientID,
	}
}

func (c *Client) VerifyIDToken(ctx context.Context, rawIDToken string) (*Identity, error) {
	if rawIDToken == "" {
		return nil, errors.New("google id token is empty")
	}

	payload, err := idtoken.Validate(ctx, rawIDToken, c.ClientID)
	if err != nil {
		return nil, fmt.Errorf("invalid google id token: %w", err)
	}

	sub, ok := payload.Claims["sub"].(string)
	if !ok || sub == "" {
		return nil, errors.New("google id token missing subject (sub)")
	}

	email, _ := payload.Claims["email"].(string)

	return &Identity{
		OAuthID: sub,
		Email:   email,
	}, nil
}
