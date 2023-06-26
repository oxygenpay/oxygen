package auth

type Config struct {
	Google GoogleConfig `yaml:"google"`
	Email  EmailConfig  `yaml:"email"`
}

type ProviderType string

const (
	ProviderTypeGoogle ProviderType = "google"
	ProviderTypeEmail  ProviderType = "email"
)

type GoogleConfig struct {
	Enabled               bool   `yaml:"enabled" env:"GOOGLE_AUTH_ENABLED" env-default:"false" env-description:"Internal variable"`
	ClientID              string `yaml:"client_id" env:"GOOGLE_AUTH_CLIENT_ID" env-description:"Internal variable"`
	ClientSecret          string `yaml:"client_secret" env:"GOOGLE_AUTH_CLIENT_SECRET" env-description:"Internal variable"`
	RedirectCallback      string `yaml:"redirect_callback" env:"GOOGLE_AUTH_REDIRECT_CALLBACK" env-description:"Internal variable"`
	AuthenticatedRedirect string `yaml:"authenticated_redirect" env:"GOOGLE_AUTH_REDIRECT_SUCCESS" env-default:"/dashboard" env-description:"Internal variable"`
}

type EmailConfig struct {
	Enabled        bool   `yaml:"enabled" env:"EMAIL_AUTH_ENABLED" env-default:"true" env-description:"Enables email auth"`
	FirstUserEmail string `yaml:"user_email" env:"EMAIL_AUTH_USER_EMAIL" env-description:"Email of an user that will be created on a first startup"`
	FirstUserPass  string `yaml:"user_password" env:"EMAIL_AUTH_USER_PASSWORD" env-description:"Password of an user that will be created on a first startup"`
}

func (c *Config) EnabledProviders() []ProviderType {
	var types []ProviderType

	if c.Email.Enabled {
		types = append(types, ProviderTypeEmail)
	}

	if c.Google.Enabled {
		types = append(types, ProviderTypeGoogle)
	}

	return types
}
