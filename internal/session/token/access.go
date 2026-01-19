package token

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type AccessClaims struct {
	UserID    string `json:"uid"`
	SessionID string `json:"sid"`

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

func (s *AccessTokenService) Generate(userID uuid.UUID, sessionId uuid.UUID, now time.Time) (string, error) {
	kid, key := s.keys.SigningKey()

	claims := AccessClaims{
		UserID:    userID.String(),
		SessionID: sessionId.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   userID.String(),
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

	if claims.Issuer != s.issuer {
		return nil, ErrInvalidClaims
	}

	return claims, nil
}
