// White-box tests for auth package.
// Uses package auth so we can access private fields (jwksURL) for testing.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"

	"github.com/bhata/AutoDreamApplier/pkg/config"
)

const wbDevSecret = "whitebox-dev-secret-32-bytes-pad"

// ─── refreshKeys / getPublicKey via mock JWKS server ─────────────────────────

// buildJWKSServer creates a test JWKS server that serves a single RSA public key.
// Returns the server, the key's "kid", and the private key for signing tokens.
func buildJWKSServer(t *testing.T) (*httptest.Server, string, *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}

	kid := "test-kid-whitebox"
	pub := &priv.PublicKey

	// Encode n and e as base64url.
	nBytes := pub.N.Bytes()
	eBytes := big.NewInt(int64(pub.E)).Bytes()
	nB64 := base64.RawURLEncoding.EncodeToString(nBytes)
	eB64 := base64.RawURLEncoding.EncodeToString(eBytes)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"keys": []map[string]interface{}{
				{"kid": kid, "kty": "RSA", "use": "sig", "n": nB64, "e": eB64},
			},
		})
	}))

	return srv, kid, priv
}

// mintRS256Token mints an RS256 token with the given private key and kid.
func mintRS256Token(kid string, privKey *rsa.PrivateKey, claims jwt.MapClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(privKey)
	if err != nil {
		panic("mintRS256Token: " + err.Error())
	}
	return signed
}

// newCognitoAuthWithJWKSURL creates a CognitoAuth pointing to a custom JWKS URL.
func newCognitoAuthWithJWKSURL(t *testing.T, jwksURL string) *CognitoAuth {
	t.Helper()
	ca := NewCognitoAuth(
		config.CognitoConfig{UserPoolID: "us-east-1_test", AppClientID: "client"},
		"us-east-1",
		zerolog.Nop(),
	)
	ca.jwksURL = jwksURL // override the private field from within the same package
	return ca
}

// TestRefreshKeys_ValidJWKS_LoadsKey verifies refreshKeys populates the keys map.
func TestRefreshKeys_ValidJWKS_LoadsKey(t *testing.T) {
	srv, kid, _ := buildJWKSServer(t)
	defer srv.Close()

	ca := newCognitoAuthWithJWKSURL(t, srv.URL)

	if err := ca.refreshKeys(context.Background()); err != nil {
		t.Fatalf("refreshKeys: %v", err)
	}

	ca.keysMu.RLock()
	_, ok := ca.keys[kid]
	ca.keysMu.RUnlock()

	if !ok {
		t.Errorf("expected key %q to be loaded after refreshKeys", kid)
	}
}

// TestGetPublicKey_CacheHit verifies that a cached key is returned without network call.
func TestGetPublicKey_CacheHit(t *testing.T) {
	// Server that fails — if getPublicKey hits network on a cached key, test fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("unexpected JWKS fetch — should have used cached key")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ca := newCognitoAuthWithJWKSURL(t, srv.URL)

	// Pre-populate the cache
	fakeKey := &rsa.PublicKey{N: big.NewInt(1), E: 65537}
	ca.keysMu.Lock()
	ca.keys["cached-kid"] = fakeKey
	ca.keysMu.Unlock()

	key, err := ca.getPublicKey(context.Background(), "cached-kid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != fakeKey {
		t.Error("expected cached key to be returned")
	}
}

// TestGetPublicKey_CacheMiss_FetchesFromJWKS verifies refreshKeys is called on miss.
func TestGetPublicKey_CacheMiss_FetchesFromJWKS(t *testing.T) {
	srv, kid, _ := buildJWKSServer(t)
	defer srv.Close()

	ca := newCognitoAuthWithJWKSURL(t, srv.URL)

	key, err := ca.getPublicKey(context.Background(), kid)
	if err != nil {
		t.Fatalf("getPublicKey: %v", err)
	}
	if key == nil {
		t.Fatal("expected non-nil key")
	}
}

// TestGetPublicKey_KeyNotInJWKS verifies an error is returned when kid not in JWKS.
func TestGetPublicKey_KeyNotInJWKS(t *testing.T) {
	srv, _, _ := buildJWKSServer(t)
	defer srv.Close()

	ca := newCognitoAuthWithJWKSURL(t, srv.URL)

	_, err := ca.getPublicKey(context.Background(), "nonexistent-kid")
	if err == nil {
		t.Fatal("expected error for missing kid, got nil")
	}
}

// TestValidateToken_ValidRS256_HappyPath verifies a valid RS256 token validates.
func TestValidateToken_ValidRS256_HappyPath(t *testing.T) {
	srv, kid, privKey := buildJWKSServer(t)
	defer srv.Close()

	ca := newCognitoAuthWithJWKSURL(t, srv.URL)

	claims := jwt.MapClaims{
		"sub":       "user-id-123",
		"email":     "rs256@example.com",
		"token_use": "id",
		"exp":       time.Now().Add(time.Hour).Unix(),
		"aud":       "client",
	}
	tokenStr := mintRS256Token(kid, privKey, claims)

	got, err := ca.ValidateToken(context.Background(), tokenStr)
	if err != nil {
		t.Fatalf("expected success for valid RS256 token, got: %v", err)
	}
	if got.TokenUse != "id" {
		t.Errorf("expected token_use 'id', got %q", got.TokenUse)
	}
}

// TestValidateToken_RS256_InvalidTokenUse verifies invalid token_use returns error.
func TestValidateToken_RS256_InvalidTokenUse(t *testing.T) {
	srv, kid, privKey := buildJWKSServer(t)
	defer srv.Close()

	ca := newCognitoAuthWithJWKSURL(t, srv.URL)

	claims := jwt.MapClaims{
		"sub":       "user-id-123",
		"email":     "rs256@example.com",
		"token_use": "refresh", // invalid
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	tokenStr := mintRS256Token(kid, privKey, claims)

	_, err := ca.ValidateToken(context.Background(), tokenStr)
	if err == nil {
		t.Fatal("expected error for invalid token_use, got nil")
	}
}

// TestRefreshKeys_InvalidJSON_ReturnsError verifies malformed JWKS response is handled.
func TestRefreshKeys_InvalidJSON_ReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json {{{"))
	}))
	defer srv.Close()

	ca := newCognitoAuthWithJWKSURL(t, srv.URL)

	err := ca.refreshKeys(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JWKS JSON, got nil")
	}
}

// TestRefreshKeys_NetworkError_ReturnsError verifies an unreachable server returns an error.
func TestRefreshKeys_NetworkError_ReturnsError(t *testing.T) {
	ca := newCognitoAuthWithJWKSURL(t, "http://127.0.0.1:19998/jwks") // unreachable

	err := ca.refreshKeys(context.Background())
	if err == nil {
		t.Fatal("expected error for unreachable JWKS server, got nil")
	}
}

// TestPeekAlg_HS256_Direct tests peekAlg directly (white-box).
func TestPeekAlg_HS256_Direct(t *testing.T) {
	// Build an HS256 token and check peekAlg returns "HS256"
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "test"})
	signed, _ := token.SignedString([]byte("secret"))

	alg, err := peekAlg(signed)
	if err != nil {
		t.Fatalf("peekAlg: %v", err)
	}
	if alg != "HS256" {
		t.Errorf("expected HS256, got %q", alg)
	}
}

// TestPeekAlg_RS256_Direct tests peekAlg returns RS256.
func TestPeekAlg_RS256_Direct(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"x"}`))
	tokenStr := header + "." + payload + ".fakesig"

	alg, err := peekAlg(tokenStr)
	if err != nil {
		t.Fatalf("peekAlg: %v", err)
	}
	if alg != "RS256" {
		t.Errorf("expected RS256, got %q", alg)
	}
}

// TestPeekAlg_InvalidBase64 tests peekAlg with invalid base64 in header.
func TestPeekAlg_InvalidBase64(t *testing.T) {
	// Header part is not valid base64
	tokenStr := "!!!invalid_base64!!!.payload.sig"
	alg, err := peekAlg(tokenStr)
	if err == nil {
		t.Errorf("expected error for invalid base64 header, got alg=%q", alg)
	}
}

// TestPeekAlg_TooFewParts tests peekAlg with too few parts.
func TestPeekAlg_TooFewParts(t *testing.T) {
	alg, err := peekAlg("onlyonepart")
	if err == nil {
		t.Errorf("expected error for too few parts, got alg=%q", alg)
	}
}

// TestExtractBearerToken tests extractBearerToken directly.
func TestExtractBearerToken_Valid(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer test-token-123")

	token := extractBearerToken(req)
	if token != "test-token-123" {
		t.Errorf("expected 'test-token-123', got %q", token)
	}
}

func TestExtractBearerToken_MissingHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	token := extractBearerToken(req)
	if token != "" {
		t.Errorf("expected empty string, got %q", token)
	}
}

func TestExtractBearerToken_InvalidFormat(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")

	token := extractBearerToken(req)
	if token != "" {
		t.Errorf("expected empty string for non-bearer token, got %q", token)
	}
}

func TestExtractBearerToken_BearerCaseInsensitive(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "BEARER my-token")

	token := extractBearerToken(req)
	if token != "my-token" {
		t.Errorf("expected 'my-token', got %q", token)
	}
}

// TestValidateDevToken_InvalidMapClaims tests validateDevToken when token claims
// are valid but token_use is invalid.
func TestValidateDevToken_Whitebox_MissingTokenUse(t *testing.T) {
	ca := &CognitoAuth{devSecret: wbDevSecret, keys: make(map[string]*rsa.PublicKey)}

	claims := jwt.MapClaims{
		"sub":   "dev:test@example.com",
		"email": "test@example.com",
		// token_use missing
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := token.SignedString([]byte(wbDevSecret))

	_, err := ca.validateDevToken(signed)
	if err == nil {
		t.Fatal("expected error for missing token_use, got nil")
	}
}

// TestLogin_FindPasswordHashError tests Login when FindPasswordHash returns empty hash.
// This exercises the "hash == ”" branch.
func TestMiddleware_InvalidBearerFormat(t *testing.T) {
	ca := &CognitoAuth{
		devSecret: wbDevSecret,
		keys:      make(map[string]*rsa.PublicKey),
		log:       zerolog.Nop(),
	}
	mw := ca.Middleware()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test with malformed Authorization header (no space)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "malformedtoken")
	rr := httptest.NewRecorder()
	mw(inner).ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for malformed auth header, got %d", rr.Code)
	}
}
