package token

import (
	"time"

	"github.com/authara-org/authara/internal/session/roles"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Audience string

const (
	AudienceApp   Audience = "app"
	AudienceAdmin Audience = "admin"
)

type AccessClaims struct {
	SessionID uuid.UUID    `json:"sid"`
	Roles     []roles.Role `json:"roles"`

	jwt.RegisteredClaims
}

type AccessTokenService struct {
	keys   *KeySet
	issuer string
	ttl    time.Duration
}

func NewAccessTokenService(
	keys *KeySet,
	issuer string,
	ttl time.Duration,
) *AccessTokenService {
	return &AccessTokenService{
		keys:   keys,
		issuer: issuer,
		ttl:    ttl,
	}
}

func (s *AccessTokenService) Generate(userID uuid.UUID, sessionId uuid.UUID, audience Audience, roles roles.Roles, now time.Time) (string, error) {
	kid, key := s.keys.SigningKey()

	claims := AccessClaims{
		SessionID: sessionId,
		Roles:     roles.List(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID.String(),
			Audience:  jwt.ClaimStrings{string(audience)},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = kid

	signed, err := token.SignedString(key)
	if err != nil {
		return "", err
	}

	return signed, nil
}

func (s *AccessTokenService) Parse(tokenString string, now time.Time) (*AccessClaims, error) {
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Name}),
		jwt.WithAudience("app", "admin"),
		jwt.WithIssuer(s.issuer),
	)

	token, err := parser.ParseWithClaims(
		tokenString,
		&AccessClaims{},
		func(t *jwt.Token) (any, error) {
			kid, ok := t.Header["kid"].(string)
			if !ok {
				return nil, ErrUnknownKey
			}

			key, ok := s.keys.VerificationKey(kid)
			if !ok {
				return nil, ErrUnknownKey
			}

			return key, nil
		},
	)

	if err != nil {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, ErrInvalidClaims
	}

	if claims.ExpiresAt == nil || claims.ExpiresAt.Time.Before(now) {
		return nil, ErrExpiredToken
	}

	if claims.Subject == "" || claims.SessionID == uuid.Nil {
		return nil, ErrInvalidClaims
	}

	return claims, nil
}
