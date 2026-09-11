package accessauth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

const AccessJWTHeader = "Cf-Access-Jwt-Assertion"

var ErrAccessDenied = errors.New("access denied")

type Identity struct {
	Email string
}

type TokenValidator interface {
	Validate(context.Context, string) (Identity, error)
}

type Config struct {
	TeamDomain string
	Audience   string
	AdminEmail string
}

type CloudflareValidator struct {
	verifier   *oidc.IDTokenVerifier
	adminEmail string
}

func ConfigFromEnvironment() Config {
	return Config{
		TeamDomain: os.Getenv("REACTORLAB_ACCESS_TEAM_DOMAIN"),
		Audience:   os.Getenv("REACTORLAB_ACCESS_AUDIENCE"),
		AdminEmail: os.Getenv("REACTORLAB_ACCESS_ADMIN_EMAIL"),
	}
}

func NewCloudflareValidator(config Config) (TokenValidator, error) {
	teamDomain, err := normalizedTeamDomain(config.TeamDomain)
	if err != nil {
		return nil, err
	}
	audience := strings.TrimSpace(config.Audience)
	if audience == "" {
		return nil, errors.New("REACTORLAB_ACCESS_AUDIENCE is required")
	}
	adminEmail := normalizedEmail(config.AdminEmail)
	if adminEmail == "" {
		return nil, errors.New("REACTORLAB_ACCESS_ADMIN_EMAIL is required")
	}

	keySet := oidc.NewRemoteKeySet(
		context.Background(),
		teamDomain+"/cdn-cgi/access/certs",
	)
	return NewCloudflareValidatorWithKeySet(
		teamDomain,
		audience,
		adminEmail,
		keySet,
	), nil
}

func NewCloudflareValidatorWithKeySet(
	issuer string,
	audience string,
	adminEmail string,
	keySet oidc.KeySet,
) *CloudflareValidator {
	return &CloudflareValidator{
		verifier: oidc.NewVerifier(
			issuer,
			keySet,
			&oidc.Config{ClientID: audience},
		),
		adminEmail: normalizedEmail(adminEmail),
	}
}

func (validator *CloudflareValidator) Validate(
	ctx context.Context,
	rawToken string,
) (Identity, error) {
	rawToken = strings.TrimSpace(rawToken)
	if rawToken == "" {
		return Identity{}, ErrAccessDenied
	}
	token, err := validator.verifier.Verify(ctx, rawToken)
	if err != nil {
		return Identity{}, fmt.Errorf("verify Access token: %w", err)
	}

	var claims struct {
		Email string `json:"email"`
	}
	if err := token.Claims(&claims); err != nil {
		return Identity{}, fmt.Errorf("read Access token claims: %w", err)
	}

	email := normalizedEmail(claims.Email)
	if email == "" || email != validator.adminEmail {
		return Identity{}, ErrAccessDenied
	}
	return Identity{Email: email}, nil
}

func normalizedTeamDomain(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil ||
		parsed.Scheme != "https" ||
		parsed.Host == "" ||
		parsed.User != nil ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return "", errors.New(
			"REACTORLAB_ACCESS_TEAM_DOMAIN must be an HTTPS origin",
		)
	}
	return "https://" + parsed.Host, nil
}

func normalizedEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
