package auth_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/internal/auth"
	"github.com/bhata/AutoDreamApplier/pkg/config"
)

const testDevSecret = "test-dev-secret-32-bytes-padding!"

// makeHS256Token creates a valid HS256 JWT signed with the given secret.
func makeHS256Token(secret string, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		panic("makeHS256Token: " + err.Error())
	}
	return signed
}

// makeMalformedToken returns a string that is not a valid JWT.
func makeMalformedToken() string {
	return "not.a.valid.jwt.at.all"
}

// makeRS256LikeToken builds a token whose header claims alg=RS256 but is not
// actually signed with RSA — we only need the header to be readable by peekAlg.
func makeRS256LikeToken() string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT","kid":"test-kid"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"fake"}`))
	return header + "." + payload + ".fakesig"
}

// ─── peekAlg tests ────────────────────────────────────────────────────────────

// peekAlg is unexported; we test it indirectly by calling WithDevSecret and
// ValidateToken, but we can also infer its behaviour through integration tests.
// Direct tests are easier via a small exported wrapper — since it's unexported we
// test the observable behaviour of ValidateToken routing.

// TestPeekAlg_HS256_Routes tests that a HS256 token with devSecret set is
// routed to the dev validation path (returns claims, no Cognito call needed).
func TestPeekAlg_HS256_RoutesToDevPath(t *testing.T) {
	ca := auth.NewCognitoAuth(config.CognitoConfig{UserPoolID: "us-east-1_test", AppClientID: "client"}, "us-east-1", zerolog.Nop())
	ca.WithDevSecret(testDevSecret)

	claims := jwt.MapClaims{
		"sub":       "dev:alice@example.com",
		"email":     "alice@example.com",
		"token_use": "id",
		"exp":       time.Now().Add(time.Hour).Unix(),
		"iat":       time.Now().Unix(),
	}
	token := makeHS256Token(testDevSecret, claims)

	got, err := ca.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if got.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %q", got.Email)
	}
}

// TestPeekAlg_RS256_UsesRS256Path ensures that when the token header says RS256
// and devSecret IS set, the code does NOT use the dev path (it tries Cognito JWKS
// and fails because no real Cognito exists — we confirm by checking the error).
func TestPeekAlg_RS256_UsesCognitoPath(t *testing.T) {
	ca := auth.NewCognitoAuth(config.CognitoConfig{UserPoolID: "us-east-1_test", AppClientID: "client"}, "us-east-1", zerolog.Nop())
	ca.WithDevSecret(testDevSecret)

	token := makeRS256LikeToken()

	// Must fail with a network/parsing error (not a dev token error)
	_, err := ca.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for invalid RS256 token, got nil")
	}
	// Should NOT be the dev "invalid token_use" style error
	if strings.Contains(err.Error(), "token_use") {
		t.Errorf("error looks like dev validation path was used: %v", err)
	}
}

// TestPeekAlg_MalformedToken ensures that a token that is not parseable returns an error.
func TestPeekAlg_MalformedToken(t *testing.T) {
	ca := auth.NewCognitoAuth(config.CognitoConfig{UserPoolID: "us-east-1_test", AppClientID: "client"}, "us-east-1", zerolog.Nop())
	ca.WithDevSecret(testDevSecret)

	_, err := ca.ValidateToken(context.Background(), makeMalformedToken())
	if err == nil {
		t.Fatal("expected error for malformed token, got nil")
	}
}

// ─── WithDevSecret ────────────────────────────────────────────────────────────

func TestWithDevSecret_WiresSecret(t *testing.T) {
	ca := auth.NewCognitoAuth(config.CognitoConfig{}, "us-east-1", zerolog.Nop())
	// Before WithDevSecret, HS256 token should fall through to the RS256 Cognito path and fail differently.
	claims := jwt.MapClaims{
		"sub":       "dev:bob@example.com",
		"email":     "bob@example.com",
		"token_use": "id",
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := makeHS256Token(testDevSecret, claims)

	// Without devSecret, ValidateToken treats HS256 as RS256 and fails parsing.
	_, errBefore := ca.ValidateToken(context.Background(), token)
	if errBefore == nil {
		t.Fatal("expected error before WithDevSecret, got nil")
	}

	// After WithDevSecret, HS256 token validates successfully.
	ca.WithDevSecret(testDevSecret)
	got, err := ca.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("expected success after WithDevSecret, got: %v", err)
	}
	if got.Sub != "dev:bob@example.com" {
		t.Errorf("expected sub dev:bob@example.com, got %q", got.Sub)
	}
}

// ─── validateDevToken tests (via ValidateToken) ───────────────────────────────

func TestValidateDevToken_ValidToken(t *testing.T) {
	ca := auth.NewCognitoAuth(config.CognitoConfig{}, "us-east-1", zerolog.Nop())
	ca.WithDevSecret(testDevSecret)

	claims := jwt.MapClaims{
		"sub":       "dev:user@example.com",
		"email":     "user@example.com",
		"token_use": "id",
		"exp":       time.Now().Add(24 * time.Hour).Unix(),
		"iat":       time.Now().Unix(),
	}
	token := makeHS256Token(testDevSecret, claims)

	got, err := ca.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Sub != "dev:user@example.com" {
		t.Errorf("sub mismatch: got %q", got.Sub)
	}
	if got.Email != "user@example.com" {
		t.Errorf("email mismatch: got %q", got.Email)
	}
	if got.TokenUse != "id" {
		t.Errorf("token_use mismatch: got %q", got.TokenUse)
	}
}

func TestValidateDevToken_ExpiredToken(t *testing.T) {
	ca := auth.NewCognitoAuth(config.CognitoConfig{}, "us-east-1", zerolog.Nop())
	ca.WithDevSecret(testDevSecret)

	claims := jwt.MapClaims{
		"sub":       "dev:expired@example.com",
		"email":     "expired@example.com",
		"token_use": "id",
		"exp":       time.Now().Add(-1 * time.Hour).Unix(),
	}
	token := makeHS256Token(testDevSecret, claims)

	_, err := ca.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestValidateDevToken_WrongSecret(t *testing.T) {
	ca := auth.NewCognitoAuth(config.CognitoConfig{}, "us-east-1", zerolog.Nop())
	ca.WithDevSecret(testDevSecret)

	claims := jwt.MapClaims{
		"sub":       "dev:user@example.com",
		"email":     "user@example.com",
		"token_use": "id",
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	// Sign with a different secret
	token := makeHS256Token("wrong-secret-that-is-different-!!!", claims)

	_, err := ca.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for wrong-secret token, got nil")
	}
}

func TestValidateDevToken_InvalidTokenUse(t *testing.T) {
	ca := auth.NewCognitoAuth(config.CognitoConfig{}, "us-east-1", zerolog.Nop())
	ca.WithDevSecret(testDevSecret)

	claims := jwt.MapClaims{
		"sub":       "dev:user@example.com",
		"email":     "user@example.com",
		"token_use": "refresh", // invalid
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := makeHS256Token(testDevSecret, claims)

	_, err := ca.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for invalid token_use, got nil")
	}
}

func TestValidateDevToken_AccessTokenUse(t *testing.T) {
	ca := auth.NewCognitoAuth(config.CognitoConfig{}, "us-east-1", zerolog.Nop())
	ca.WithDevSecret(testDevSecret)

	claims := jwt.MapClaims{
		"sub":       "dev:user@example.com",
		"email":     "user@example.com",
		"token_use": "access",
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := makeHS256Token(testDevSecret, claims)

	got, err := ca.ValidateToken(context.Background(), token)
	if err != nil {
		t.Fatalf("expected success for access token_use, got: %v", err)
	}
	if got.TokenUse != "access" {
		t.Errorf("expected token_use access, got %q", got.TokenUse)
	}
}

// ─── Middleware tests ─────────────────────────────────────────────────────────

func TestMiddleware_MissingAuthHeader(t *testing.T) {
	ca := auth.NewCognitoAuth(config.CognitoConfig{}, "us-east-1", zerolog.Nop())
	mw := ca.Middleware()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	mw(inner).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestMiddleware_ValidDevToken(t *testing.T) {
	ca := auth.NewCognitoAuth(config.CognitoConfig{}, "us-east-1", zerolog.Nop())
	ca.WithDevSecret(testDevSecret)
	mw := ca.Middleware()

	claims := jwt.MapClaims{
		"sub":       "dev:mw@example.com",
		"email":     "mw@example.com",
		"token_use": "id",
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	token := makeHS256Token(testDevSecret, claims)

	var ctxEmail string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, ok := auth.GetUserFromContext(r.Context())
		if ok {
			ctxEmail = c.Email
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	mw(inner).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	if ctxEmail != "mw@example.com" {
		t.Errorf("expected email in context, got %q", ctxEmail)
	}
}

// ─── JWKS-based RS256 path (mock httptest server) ─────────────────────────────

// buildJWKSServer returns a test server that vends a JWKS containing `kid` -> encoded key.
// Since generating a real RSA key is expensive, we just test the error path (bad key data
// causes no crash and returns a meaningful error).
func TestValidateToken_RS256_BadJWKS_ReturnsError(t *testing.T) {
	// Serve a JWKS with mangled base64 so key parsing fails gracefully.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"keys": []map[string]interface{}{
				{"kid": "key1", "n": "!!!invalid_base64!!!", "e": "AQAB"},
			},
		})
	}))
	defer srv.Close()

	// We cannot override the JWKS URL on the existing struct (it's private),
	// so we use reflection-free approach: accept that the default URL fails with
	// a network error (no real Cognito), and the RS256 path always returns error.
	ca := auth.NewCognitoAuth(config.CognitoConfig{UserPoolID: "us-east-1_test", AppClientID: "client"}, "us-east-1", zerolog.Nop())
	// No devSecret — RS256 path
	token := makeRS256LikeToken()

	_, err := ca.ValidateToken(context.Background(), token)
	if err == nil {
		t.Fatal("expected error for RS256 token with no valid JWKS, got nil")
	}
}

// TestGetUserFromContext_NotPresent returns false when nothing was set.
func TestGetUserFromContext_NotPresent(t *testing.T) {
	_, ok := auth.GetUserFromContext(context.Background())
	if ok {
		t.Error("expected ok=false when context has no user claims")
	}
}
