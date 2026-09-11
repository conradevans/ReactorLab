package accessauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

type testKeySet struct {
	key *rsa.PublicKey
}

func (keySet testKeySet) VerifySignature(
	_ context.Context,
	rawJWT string,
) ([]byte, error) {
	signed, err := jose.ParseSigned(
		rawJWT,
		[]jose.SignatureAlgorithm{jose.RS256},
	)
	if err != nil {
		return nil, err
	}
	return signed.Verify(keySet.key)
}

func TestCloudflareValidatorRequiresValidClaimsAndAdminEmail(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: privateKey},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		t.Fatal(err)
	}

	const (
		issuer   = "https://test.cloudflareaccess.com"
		audience = "reactorlab-audience"
		email    = "admin@example.com"
	)
	validator := NewCloudflareValidatorWithKeySet(
		issuer,
		audience,
		email,
		testKeySet{key: &privateKey.PublicKey},
	)

	token := signedTestToken(t, signer, issuer, audience, email)
	identity, err := validator.Validate(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if identity.Email != email {
		t.Fatalf("email = %q, want %q", identity.Email, email)
	}

	wrongEmail := signedTestToken(
		t,
		signer,
		issuer,
		audience,
		"other@example.com",
	)
	if _, err := validator.Validate(
		context.Background(),
		wrongEmail,
	); err == nil {
		t.Fatal("expected wrong administrator email to be rejected")
	}
}

func signedTestToken(
	t *testing.T,
	signer jose.Signer,
	issuer string,
	audience string,
	email string,
) string {
	t.Helper()
	claims := jwt.Claims{
		Issuer:    issuer,
		Audience:  jwt.Audience{audience},
		Expiry:    jwt.NewNumericDate(time.Now().Add(time.Hour)),
		NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	custom := struct {
		Email string `json:"email"`
	}{Email: email}
	token, err := jwt.Signed(signer).
		Claims(claims).
		Claims(custom).
		Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return token
}
