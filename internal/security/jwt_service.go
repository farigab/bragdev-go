// Package security provides JWT helper services used for authentication.
package security

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/farigab/bragdoc/internal/logger"
)

// TokenService defines the JWT operations required by the application.
type TokenService interface {
	// GenerateToken creates a signed token for the specified user login and extra claims.
	GenerateToken(userLogin string, extraClaims map[string]interface{}) (string, error)
	// ExtractUserLogin returns the login stored in the token or an error.
	ExtractUserLogin(tokenStr string) (string, error)
	// IsValid returns true if the token is valid and signature checks out.
	IsValid(tokenStr string) bool
	// IsExpired returns true if the token is expired or invalid.
	IsExpired(tokenStr string) bool
}

// JWTService implements TokenService using HMAC SHA256.
type JWTService struct {
	secret                []byte
	accessTokenExpiration time.Duration
}

// NewJWTService creates a JWTService configured with the given secret and expiration.
func NewJWTService(secret string, accessTokenExpirationSeconds int64) *JWTService {
	if secret == "" {
		logger.Errorf("JWT_SECRET environment variable must be set")
		os.Exit(1)
	}
	return &JWTService{
		secret:                []byte(secret),
		accessTokenExpiration: time.Duration(accessTokenExpirationSeconds) * time.Second,
	}
}

// GenerateToken builds and signs a JWT for the provided user login.
func (s *JWTService) GenerateToken(userLogin string, extraClaims map[string]interface{}) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{}
	claims["login"] = userLogin
	for k, v := range extraClaims {
		claims[k] = v
	}
	claims["iat"] = now.Unix()
	claims["exp"] = now.Add(s.accessTokenExpiration).Unix()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// parseToken attempts to parse the token and returns claims if present.
func (s *JWTService) parseToken(tokenStr string) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return s.secret, nil
	})

	if err != nil {
		// If token parsed but validation failed (e.g., expired), token may be non-nil
		if token != nil {
			if claims, ok := token.Claims.(jwt.MapClaims); ok {
				return claims, err
			}
		}
		return nil, err
	}

	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		return claims, nil
	}
	return nil, errors.New("invalid token claims")
}

// ExtractUserLogin extracts the `login` claim from the token or returns an error.
func (s *JWTService) ExtractUserLogin(tokenStr string) (string, error) {
	claims, err := s.parseToken(tokenStr)
	if err != nil {
		return "", err
	}
	if login, ok := claims["login"].(string); ok {
		return login, nil
	}
	return "", errors.New("login claim missing")
}

// IsValid returns true when the token parses and validates correctly.
func (s *JWTService) IsValid(tokenStr string) bool {
	_, err := s.parseToken(tokenStr)
	return err == nil
}

// IsExpired returns true when token is expired or cannot be parsed.
func (s *JWTService) IsExpired(tokenStr string) bool {
	claims, _ := s.parseToken(tokenStr)
	if claims == nil {
		return true
	}
	if exp, ok := claims["exp"].(float64); ok {
		return time.Unix(int64(exp), 0).Before(time.Now())
	}
	return true
}

// ExtractUserLoginSafe attempts to parse the token and returns the login claim
// when present even if the token failed validation (e.g., expired token).
func (s *JWTService) ExtractUserLoginSafe(tokenStr string) (string, error) {
	claims, err := s.parseToken(tokenStr)
	// If parsing returned claims despite an error (for example an expired token),
	// prefer returning the `login` claim when present instead of failing outright.
	if claims == nil {
		return "", err
	}
	if login, ok := claims["login"].(string); ok && login != "" {
		return login, nil
	}
	if err != nil {
		return "", err
	}
	return "", errors.New("login claim missing")
}
