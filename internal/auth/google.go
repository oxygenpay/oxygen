package auth

import (
	"context"
	"encoding/json"
	"io"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleConfig struct {
	ClientID              string `yaml:"client_id" env:"GOOGLE_AUTH_CLIENT_ID" env-description:"Internal variable"`
	ClientSecret          string `yaml:"client_secret" env:"GOOGLE_AUTH_CLIENT_SECRET" env-description:"Internal variable"`
	RedirectCallback      string `yaml:"redirect_callback" env:"GOOGLE_AUTH_REDIRECT_CALLBACK" env-description:"Internal variable"`
	AuthenticatedRedirect string `json:"authenticated_redirect" env:"GOOGLE_AUTH_REDIRECT_SUCCESS" env-default:"/" env-description:"Internal variable"`
}

type GoogleOAuthManager struct {
	config                   *oauth2.Config
	authenticatedRedirectURL string
	logger                   *zerolog.Logger
}

type GoogleUser struct {
	Sub           string `json:"sub"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Locale        string `json:"locale"`
}

const googleUserEndpoint = "https://www.googleapis.com/oauth2/v3/userinfo"

func NewGoogleOAuth(cfg GoogleConfig, logger *zerolog.Logger) *GoogleOAuthManager {
	log := logger.With().Str("channel", "google_oauth").Logger()

	return &GoogleOAuthManager{
		config: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectCallback,
			Scopes:       []string{"openid", "profile", "email"},
			Endpoint:     google.Endpoint,
		},
		authenticatedRedirectURL: cfg.AuthenticatedRedirect,
		logger:                   &log,
	}
}

// RedirectURL return URL to Google auth screen.
func (a *GoogleOAuthManager) RedirectURL() string {
	return a.config.AuthCodeURL("")
}

func (a *GoogleOAuthManager) ResolveUser(ctx context.Context, code string) (*GoogleUser, error) {
	token, err := a.config.Exchange(ctx, code)
	if err != nil {
		return nil, errors.Wrap(err, "unable to exchange code for token")
	}

	resp, err := a.config.Client(ctx, token).Get(googleUserEndpoint)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get user data")
	}

	rawUser, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read user")
	}

	user := &GoogleUser{}
	if err := json.Unmarshal(rawUser, user); err != nil {
		return nil, errors.Wrap(err, "unable to unmarshal user")
	}

	return user, nil
}

// GetAuthenticatedRedirectURL returns a URL where user
// should be redirected when already authenticated.
func (a *GoogleOAuthManager) GetAuthenticatedRedirectURL() string {
	return a.authenticatedRedirectURL
}
