package tatum

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	kms "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/registry"
	"github.com/oxygenpay/tatum-sdk/tatum"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Config struct {
	BasePath   string
	APIKey     string `yaml:"api_key" env:"TATUM_API_KEY" env-description:"Tatum API Key"`
	TestAPIKey string `yaml:"test_api_key" env:"TATUM_TEST_API_KEY" env-description:"Tatum Test API Key"`
	HMACSecret string `yaml:"tatum_hmac_secret" env:"TATUM_HMAC_SECRET" env-description:"Tatum HMAC Secret. Use any random string with 8+ chars"`

	// HMACForceSet will make "set hmac set" request on every service start. Useful if HMAC secret was changed.
	HMACForceSet bool `yaml:"tatum_hmac_force_set" env:"TATUM_HMAC_FORCE_SET" env-description:"Internal variable"`
}

type Provider struct {
	config   Config
	registry *registry.Service
	logger   *zerolog.Logger

	mainClient *tatum.APIClient
	testClient *tatum.APIClient
}

const (
	TokenHeader = "x-api-key"
	EthTestnet  = "ethereum-goerli"

	userAgent = "Go-http/1.1"

	subscriptionTypeAddressTX = "ADDRESS_TRANSACTION"

	registryHMACEnabledMainnet = "tatum.hmac_enabled.mainnet"
	registryHMACEnabledTestnet = "tatum.hmac_enabled.testnet"
)

func New(config Config, registryService *registry.Service, logger *zerolog.Logger) *Provider {
	if config.BasePath == "" {
		config.BasePath = "https://api-eu1.tatum.io"
	}

	setup := func(apiKey string) *tatum.APIClient {
		cfg := tatum.NewConfiguration()
		cfg.UserAgent = userAgent
		cfg.BasePath = config.BasePath

		cfg.AddDefaultHeader(TokenHeader, apiKey)

		return tatum.NewAPIClient(cfg)
	}

	log := logger.With().Str("channel", "tatum_provider").Logger()

	p := &Provider{
		config:     config,
		registry:   registryService,
		logger:     &log,
		mainClient: setup(config.APIKey),
		testClient: setup(config.TestAPIKey),
	}

	if config.HMACSecret != "" {
		p.ensureHMAC(config.HMACSecret, config.HMACForceSet)
	}

	return p
}

func (p *Provider) Main() *tatum.APIClient {
	return p.mainClient
}

func (p *Provider) Test() *tatum.APIClient {
	return p.testClient
}

type SubscriptionParams struct {
	Blockchain money.Blockchain
	Address    string
	IsTest     bool
	WebhookURL string
}

type SubscriptionResponse struct {
	ID string `json:"id"`
}

// SubscribeToWebhook auto-generated sdk throws an error on this request, so it's rewritten manually.
func (p *Provider) SubscribeToWebhook(ctx context.Context, params SubscriptionParams) (string, error) {
	url := fmt.Sprintf("%s/v3/subscription", p.config.BasePath)

	token := p.config.APIKey
	if params.IsTest {
		token = p.config.TestAPIKey

		if params.Blockchain.String() == string(kms.ETH) {
			url += "?testnetType=" + EthTestnet
		}
	}

	reqBody, err := json.Marshal(tatum.CreateSubscriptionNotification{
		Type_: subscriptionTypeAddressTX,
		Attr: &tatum.CreateSubscriptionNotificationAttr{
			Address: params.Address,
			Chain:   params.Blockchain.String(),
			Url:     params.WebhookURL,
		},
	})

	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}

	req.Header.Set(TokenHeader, token)
	req.Header.Set("User-Agent", token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	if res.StatusCode != http.StatusOK {
		p.logger.Warn().
			Str("wallet_address", params.Address).
			Str("status_code", res.Status).
			Str("request", string(reqBody)).
			Str("response", string(resBody)).
			Msg("unable to create subscription")

		return "", errors.New("invalid tatum response code")
	}

	if err != nil {
		p.logger.Warn().Err(err).Str("wallet_address", params.Address).Msg("unable to create subscription")
		return "", errors.Wrapf(err, "unable to create subscription")
	}

	id := SubscriptionResponse{}
	if err := json.Unmarshal(resBody, &id); err != nil {
		return "", err
	}

	return id.ID, nil
}

func (p *Provider) ValidateHMAC(body []byte, hash string) bool {
	secret := p.config.HMACSecret

	// skip validation if not set
	if secret == "" {
		return true
	}

	// https://apidoc.tatum.io/tag/Notification-subscriptions#operation/enableWebHookHmac
	mac := hmac.New(sha512.New, []byte(secret))
	if _, err := mac.Write(body); err != nil {
		return false
	}

	expectedMAC := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return expectedMAC == hash
}

func (p *Provider) ensureHMAC(secret string, isForce bool) {
	ctx := context.Background()

	if err := p.enableSubscriptionSignature(ctx, secret, false, isForce); err != nil {
		p.logger.Warn().Err(err).Bool("is_force", isForce).Msgf("unable to ensure HMAC for mainnet")
	}

	if err := p.enableSubscriptionSignature(ctx, secret, true, isForce); err != nil {
		p.logger.Warn().Err(err).Bool("is_force", isForce).Msgf("unable to ensure HMAC for testnet")
	}
}

func (p *Provider) enableSubscriptionSignature(ctx context.Context, secret string, isTest, isForce bool) error {
	var client = p.Main()
	var registryKey = registryHMACEnabledMainnet
	if isTest {
		client = p.Test()
		registryKey = registryHMACEnabledTestnet
	}

	// 1. get registry key
	enabled := p.registry.GetValueSafe(ctx, registryKey, "")

	if enabled.Value != "" && !isForce {
		p.logger.Info().Str(registryKey, enabled.Value).Msg("skipping hmac request because it is already set")
		return nil
	}

	// 2. send request to enable HMAC
	res, err := client.NotificationSubscriptionsApi.EnableWebHookHmac(ctx, tatum.HmacWebHook{
		HmacSecret: secret,
	})

	if err != nil {
		return errors.Wrapf(err, "unable to set HMAC signature, response status %q", res.StatusCode)
	}

	defer res.Body.Close()

	// 3. set registry key
	enabled, err = p.registry.Set(ctx, registryKey, "true")

	if err != nil {
		return errors.Wrapf(err, "unable to enable set registry key %q", registryKey)
	}

	p.logger.Info().Str(registryKey, enabled.Value).Msg("successfully set HMAC secret")

	return nil
}
