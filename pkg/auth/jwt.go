package auth

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// JWTProvider wraps JWTService to satisfy the Provider interface.
type JWTProvider struct {
	*JWTService
}

func NewJWTProvider(svc *JWTService) Provider {
	return &JWTProvider{JWTService: svc}
}

// JWTService handles the issuance, validation, and revocation checking of
// JSON Web Tokens within the BFFX framework.
type JWTService struct {
	secret        []byte
	issuer        string
	audience      string
	requireIssAud bool
	revokedCheck  func(jti string) bool
}

// NewJWTService creates a new JWTService using the provided secret. It
// automatically configures issuer and audience from environment variables
// BFFX_JWT_ISS and BFFX_JWT_AUD.
func NewJWTService(secret string) *JWTService {
	return &JWTService{
		secret:        []byte(secret),
		issuer:        strings.TrimSpace(os.Getenv("BFFX_JWT_ISS")),
		audience:      strings.TrimSpace(os.Getenv("BFFX_JWT_AUD")),
		requireIssAud: os.Getenv("BFFX_JWT_REQUIRE_ISS_AUD") == "true",
	}
}

// SetRevocationChecker optionally rejects tokens whose jti is revoked (e.g. Redis set lookup).
func (s *JWTService) SetRevocationChecker(fn func(string) bool) {
	s.revokedCheck = fn
}

// Issuer returns the configured issuer.
func (s *JWTService) Issuer() string {
	return s.issuer
}

// GenerateToken issues an HS256 JWT. Pass anon=true for guest/session tokens (claim "anon": true).
func (s *JWTService) GenerateToken(userID, role, deviceID, fingerprint string, anon bool, duration time.Duration) (string, error) {
	jti := uuid.New().String()
	claims := jwt.MapClaims{
		"jti": jti,
		"sub": userID,
		"role": role,
		"dev":  deviceID,
		"fpt":  fingerprint,
		"exp":  time.Now().Add(duration).Unix(),
		"iat":  time.Now().Unix(),
	}
	if anon {
		claims["anon"] = true
	}
	if s.issuer != "" {
		claims["iss"] = s.issuer
	}
	if s.audience != "" {
		claims["aud"] = s.audience
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secret))
}

func (s *JWTService) Type() string {
	return "builtin"
}

func (s *JWTService) UserIDFromClaims(claims map[string]any) string {
	if sub, ok := claims["sub"].(string); ok {
		return sub
	}
	return ""
}

func (s *JWTService) ValidateToken(ctx context.Context, tokenString string) (map[string]any, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(s.secret), nil
	})

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}

	if err := s.validateStandardClaims(claims); err != nil {
		return nil, err
	}

	out := make(map[string]any, len(claims))
	for k, v := range claims {
		out[k] = v
	}
	return out, nil
}

func (s *JWTService) validateStandardClaims(claims jwt.MapClaims) error {
	jti, _ := claims["jti"].(string)
	if jti != "" && s.revokedCheck != nil {
		if s.revokedCheck(jti) {
			return errors.New("token revoked")
		}
	}

	if s.issuer != "" {
		iss, _ := claims["iss"].(string)
		if s.requireIssAud {
			if iss == "" {
				return errors.New("missing iss claim")
			}
			if iss != s.issuer {
				return errors.New("invalid issuer")
			}
		} else if iss != "" && iss != s.issuer {
			return errors.New("invalid issuer")
		}
	}

	if s.audience != "" {
		rawAud := claims["aud"]
		if s.requireIssAud {
			if !audienceMatches(rawAud, s.audience) {
				return errors.New("invalid audience")
			}
		} else if rawAud != nil && !audienceMatches(rawAud, s.audience) {
			return errors.New("invalid audience")
		}
	}

	return nil
}

func audienceMatches(raw any, want string) bool {
	switch v := raw.(type) {
	case string:
		return v == want
	case []any:
		for _, x := range v {
			if s, ok := x.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}

func bcryptCost() int {
	const defaultCost = 12
	s := strings.TrimSpace(os.Getenv("BFFX_BCRYPT_COST"))
	if s == "" {
		return defaultCost
	}
	c, err := strconv.Atoi(s)
	if err != nil {
		return defaultCost
	}
	if c < 10 {
		return defaultCost
	}
	if c > 15 {
		return 15
	}
	return c
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost())
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
