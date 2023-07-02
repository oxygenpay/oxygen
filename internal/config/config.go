package config

import (
	"io"
	"sort"
	"strings"
	"sync"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/olekukonko/tablewriter"
	"github.com/oxygenpay/oxygen/internal/auth"
	"github.com/oxygenpay/oxygen/internal/db/connection/bolt"
	"github.com/oxygenpay/oxygen/internal/db/connection/pg"
	"github.com/oxygenpay/oxygen/internal/log"
	"github.com/oxygenpay/oxygen/internal/provider/tatum"
	"github.com/oxygenpay/oxygen/internal/provider/trongrid"
	"github.com/oxygenpay/oxygen/internal/server/http"
	"github.com/oxygenpay/oxygen/internal/service/processing"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/oxygenpay/oxygen/pkg/api-kms/v1/client"
	"github.com/samber/lo"
)

type Config struct {
	// compile-time parameters
	GitCommit     string
	GitVersion    string
	EmbedFrontend bool

	Env    string     `yaml:"env" env:"APP_ENV" env-default:"production" env-description:"Environment [production, local, sandbox]"`
	Debug  bool       `yaml:"debug" env:"APP_DEBUG" env-default:"false" env-description:"Enables debug mode"`
	Logger log.Config `yaml:"logger"`

	Oxygen Oxygen `yaml:"oxygen"`
	KMS    KMS    `yaml:"kms"`

	Providers Providers `yaml:"providers"`

	Notifications Notifications `yaml:"notifications"`
}

type Oxygen struct {
	Server     http.Config       `yaml:"server"`
	Auth       auth.Config       `yaml:"auth"`
	Postgres   pg.Config         `yaml:"postgres"`
	Processing processing.Config `yaml:"processing"`
}

type KMS struct {
	// IsEmbedded indicates that app is running in 'all-in-one' mode.
	// Not suitable for safety reasons as KMS should operate in isolated environment in order to
	// keep private keys secure.
	IsEmbedded bool `yaml:"-"`

	Server http.Config `yaml:"server"`
	Bolt   bolt.Config `yaml:"store"`
}

type Providers struct {
	Tatum     tatum.Config    `yaml:"tatum"`
	Trongrid  trongrid.Config `yaml:"trongrid"`
	KmsClient client.Config   `yaml:"kms"`
}

type Notifications struct {
	SlackWebhookURL string `yaml:"slack_webhook_url" env:"NOTIFICATIONS_SLACK_WEBHOOK_URL" env-description:"Internal variable"`
}

var once = sync.Once{}
var cfg = &Config{}
var errCfg error

func New(gitCommit, gitVersion, configPath string, skipConfig, embedFrontend bool) (*Config, error) {
	once.Do(func() {
		cfg = &Config{
			GitCommit:     gitCommit,
			GitVersion:    gitVersion,
			EmbedFrontend: embedFrontend,
		}

		if skipConfig {
			errCfg = cleanenv.ReadEnv(cfg)
			return
		}

		errCfg = cleanenv.ReadConfig(configPath, cfg)
	})

	return cfg, errCfg
}

func PrintUsage(w io.Writer) error {
	desc, err := cleanenv.GetDescription(&Config{}, nil)
	if err != nil {
		return err
	}

	const delimiter = "||"

	// 1 line == 1 env var
	desc = strings.ReplaceAll(desc, "\n    \t", delimiter)

	lines := strings.Split(desc, "\n")

	// remove header
	lines = lines[1:]

	// hide internal vars
	lines = util.FilterSlice(lines, func(line string) bool {
		return !strings.Contains(strings.ToLower(line), "internal variable")
	})

	// remove duplicates
	lines = lo.Uniq(lines)

	// sort a-z (skip header)
	sort.Strings(lines[1:])

	// write as a table
	t := tablewriter.NewWriter(w)
	t.SetBorder(false)
	t.SetAutoWrapText(false)
	t.SetHeader([]string{"ENV", "Description"})
	t.SetHeaderAlignment(tablewriter.ALIGN_LEFT)

	for _, line := range lines {
		cells := strings.Split(line, delimiter)
		cells = util.MapSlice(cells, strings.TrimSpace)
		t.Append(cells)
	}

	t.Render()

	return nil
}
