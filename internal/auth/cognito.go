package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/pkg/config"
	"github.com/bhata/AutoDreamApplier/pkg/response"
)

// Claims represents the JWT claims from Cognito.
type Claims struct {
	jwt.RegisteredClaims
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	CognitoGroups []string `json:"cognito:groups,omitempty"`
	TokenUse      string `json:"token_use"`
}

// contextKey is a private type for context keys.
type contextKey string

const (
	UserContextKey contextKey = "user"
)

// CognitoAuth handles JWT validation against AWS Cognito.
type CognitoAuth struct {
	userPoolID  string
	appClientID string
	region      string
	jwksURL     string
	keys        map[string]*rsa.PublicKey
	keysMu      sync.RWMutex
	log         zerolog.Logger
	devSecret   string // non-empty only in non-production; enables HS256 dev token validation
}

// NewCognitoAuth creates a new Cognito authenticator.
func NewCognitoAuth(cfg config.CognitoConfig, region string, log zerolog.Logger) *CognitoAuth {
	jwksURL := fmt.Sprintf(
		"https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json",
		region, cfg.UserPoolID,
	)

	return &CognitoAuth{
		userPoolID:  cfg.UserPoolID,
		appClientID: cfg.AppClientID,
		region:      region,
		jwksURL:     jwksURL,
		keys:        make(map[string]*rsa.PublicKey),
		log:         log,
	}
}

// Middleware returns an HTTP middleware that validates Cognito JWTs.
func (ca *CognitoAuth) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearerToken(r)
			if tokenStr == "" {
				response.Unauthorized(w, "missing or invalid authorization header")
				return
			}

			claims, err := ca.ValidateToken(r.Context(), tokenStr)
			if err != nil {
				ca.log.Warn().Err(err).Msg("token validation failed")
				response.Unauthorized(w, "invalid or expired token")
				return
			}

			// Add claims to context
			ctx := context.WithValue(r.Context(), UserContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ValidateToken validates a JWT token and returns claims.
func (ca *CognitoAuth) ValidateToken(ctx context.Context, tokenStr string) (*Claims, error) {
	// Route dev HS256 tokens when devSecret is configured.
	if ca.devSecret != "" {
		if alg, _ := peekAlg(tokenStr); alg == "HS256" {
			return ca.validateDevToken(tokenStr)
		}
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing kid in token header")
		}

		key, err := ca.getPublicKey(ctx, kid)
		if err != nil {
			return nil, fmt.Errorf("getting public key: %w", err)
		}

		return key, nil
	})

	if err != nil {
		return nil, fmt.Errorf("parsing token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Verify token_use is "access" or "id"
	if claims.TokenUse != "access" && claims.TokenUse != "id" {
		return nil, fmt.Errorf("invalid token_use: %s", claims.TokenUse)
	}

	return claims, nil
}

// GetUserFromContext extracts user claims from request context.
func GetUserFromContext(ctx context.Context) (*Claims, bool) {
	claims, ok := ctx.Value(UserContextKey).(*Claims)
	return claims, ok
}

func (ca *CognitoAuth) getPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	ca.keysMu.RLock()
	if key, ok := ca.keys[kid]; ok {
		ca.keysMu.RUnlock()
		return key, nil
	}
	ca.keysMu.RUnlock()

	// Fetch JWKS
	if err := ca.refreshKeys(ctx); err != nil {
		return nil, err
	}

	ca.keysMu.RLock()
	defer ca.keysMu.RUnlock()

	key, ok := ca.keys[kid]
	if !ok {
		return nil, fmt.Errorf("key %s not found in JWKS", kid)
	}

	return key, nil
}

func (ca *CognitoAuth) refreshKeys(ctx context.Context) error {
	client := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ca.jwksURL, nil)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching JWKS: %w", err)
	}
	defer resp.Body.Close()

	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decoding JWKS: %w", err)
	}

	ca.keysMu.Lock()
	defer ca.keysMu.Unlock()

	for _, key := range jwks.Keys {
		nBytes, err := base64.RawURLEncoding.DecodeString(key.N)
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(key.E)
		if err != nil {
			continue
		}

		n := new(big.Int).SetBytes(nBytes)
		e := int(new(big.Int).SetBytes(eBytes).Int64())

		ca.keys[key.Kid] = &rsa.PublicKey{N: n, E: e}
	}

	ca.log.Debug().Int("keys_loaded", len(ca.keys)).Msg("JWKS refreshed")
	return nil
}

// WithDevSecret configures the authenticator to accept HS256 dev tokens signed with secret.
// Must only be called in non-production environments.
func (ca *CognitoAuth) WithDevSecret(secret string) {
	ca.devSecret = secret
}

// validateDevToken validates an HS256 token signed with devSecret.
func (ca *CognitoAuth) validateDevToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwt.MapClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(ca.devSecret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("parsing dev token: %w", err)
	}

	mapClaims, ok := token.Claims.(*jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid dev token claims")
	}

	sub, _ := (*mapClaims)["sub"].(string)
	email, _ := (*mapClaims)["email"].(string)
	tokenUse, _ := (*mapClaims)["token_use"].(string)

	if tokenUse != "access" && tokenUse != "id" {
		return nil, fmt.Errorf("invalid token_use: %s", tokenUse)
	}

	return &Claims{
		Sub:      sub,
		Email:    email,
		TokenUse: tokenUse,
	}, nil
}

// peekAlg reads the "alg" field from a JWT header without verifying the signature.
func peekAlg(tokenStr string) (string, error) {
	parts := strings.SplitN(tokenStr, ".", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token format")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", fmt.Errorf("decoding token header: %w", err)
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return "", fmt.Errorf("unmarshaling token header: %w", err)
	}
	return header.Alg, nil
}

func extractBearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}

	return parts[1]
}
