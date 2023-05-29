package config

import (
	"sync"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/oxygenpay/oxygen/internal/auth"
	"github.com/oxygenpay/oxygen/internal/db/connection/bolt"
	"github.com/oxygenpay/oxygen/internal/db/connection/pg"
	"github.com/oxygenpay/oxygen/internal/log"
	"github.com/oxygenpay/oxygen/internal/provider/tatum"
	"github.com/oxygenpay/oxygen/internal/provider/trongrid"
	"github.com/oxygenpay/oxygen/internal/server/http"
	"github.com/oxygenpay/oxygen/pkg/api-kms/v1/client"
)

type Config struct {
	GitCommit  string
	GitVersion string
	Env        string        `yaml:"env" env:"APP_ENV" env-default:"production"`
	Debug      bool          `yaml:"debug" env:"APP_DEBUG" env-default:"false"`
	Logger     log.Config    `yaml:"logger"`
	Web        http.Config   `yaml:"web"`
	Postgres   pg.Config     `yaml:"postgres"`
	Bolt       bolt.Config   `yaml:"bolt"`
	Auth       auth.Config   `yaml:"auth"`
	KmsClient  client.Config `yaml:"kms_client"`
	Providers  struct {
		Tatum    tatum.Config
		Trongrid trongrid.Config
	} `yaml:"providers"`
	Notifications struct {
		SlackWebhookURL string `yaml:"slack_webhook_url" env:"NOTIFICATIONS_SLACK_WEBHOOK_URL"`
	} `yaml:"notifications"`
}

var once = sync.Once{}
var cfg = &Config{}
var errCfg error

func New(gitCommit, giVersion, configPath string, skipConfig bool) (*Config, error) {
	once.Do(func() {
		cfg = &Config{GitCommit: gitCommit, GitVersion: giVersion}

		if skipConfig {
			errCfg = cleanenv.ReadEnv(cfg)
			return
		}

		errCfg = cleanenv.ReadConfig(configPath, cfg)
	})

	return cfg, errCfg
}
