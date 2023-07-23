// package processing implements methods for invoices processing
package processing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/bus"
	kmswallet "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/lock"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/provider/tatum"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type BlockchainService interface {
	blockchain.Resolver
	blockchain.Convertor
	blockchain.Broadcaster
	blockchain.FeeCalculator
}

type Service struct {
	config        Config
	wallets       *wallet.Service
	merchants     *merchant.Service
	payments      *payment.Service
	transactions  *transaction.Service
	blockchain    BlockchainService
	tatumProvider *tatum.Provider
	publisher     bus.Publisher
	locker        *lock.Locker
	logger        *zerolog.Logger
}

type Config struct {
	WebhookBasePath         string `yaml:"webhook_base_path" env:"PROCESSING_WEBHOOK_BASE_PATH" env-description:"Base path for webhooks (sub)domain. Example: https://pay.site.com"`
	PaymentFrontendBasePath string `yaml:"payment_frontend_base_path" env:"PROCESSING_PAYMENT_FRONTEND_BASE_PATH" env-description:"Base path for payment UI. Example: https://pay.site.com"`
	PaymentFrontendSubPath  string `yaml:"payment_frontend_sub_path" env:"PROCESSING_PAYMENT_FRONTEND_SUB_PATH" env-default:"/p" env-description:"Sub path for payment UI"`
	// DefaultServiceFee as float percentage. 1% is 0.01
	DefaultServiceFee float64 `yaml:"default_service_fee" env:"PROCESSING_DEFAULT_SERVICE_FEE" env-default:"0" env-description:"Internal variable"`
}

func (c *Config) PaymentFrontendPath() string {
	base := strings.TrimSuffix(c.PaymentFrontendBasePath, "/")
	sub := strings.Trim(c.PaymentFrontendSubPath, "/")

	if sub == "" {
		return base
	}

	return base + "/" + sub
}

var (
	ErrStatusInvalid         = errors.New("payment status is invalid")
	ErrPaymentOptionsMissing = errors.New("payment options are not fully fulfilled")
	ErrSignatureVerification = errors.New("unable to verify request signature")
	ErrInboundWallet         = errors.New("inbound wallet error")
)

func New(
	config Config,
	wallets *wallet.Service,
	merchants *merchant.Service,
	payments *payment.Service,
	transactions *transaction.Service,
	blockchainService BlockchainService,
	tatumProvider *tatum.Provider,
	publisher bus.Publisher,
	locker *lock.Locker,
	logger *zerolog.Logger,
) *Service {
	log := logger.With().Str("channel", "processing_service").Logger()

	return &Service{
		config:        config,
		wallets:       wallets,
		merchants:     merchants,
		payments:      payments,
		transactions:  transactions,
		blockchain:    blockchainService,
		tatumProvider: tatumProvider,
		publisher:     publisher,
		locker:        locker,
		logger:        &log,
	}
}

type DetailedPayment struct {
	Payment       *payment.Payment
	Customer      *payment.Customer
	Merchant      *merchant.Merchant
	PaymentMethod *payment.Method
	PaymentInfo   *PaymentInfo
}

// PaymentInfo represents simplified transaction information.
type PaymentInfo struct {
	Status           payment.Status
	PaymentLink      string
	RecipientAddress string

	Amount          string
	AmountFormatted string

	ExpiresAt             time.Time
	ExpirationDurationMin int64

	SuccessAction  *payment.SuccessAction
	SuccessURL     *string
	SuccessMessage *string
}

func (s *Service) GetDetailedPayment(ctx context.Context, merchantID, paymentID int64) (*DetailedPayment, error) {
	pt, err := s.payments.GetByID(ctx, merchantID, paymentID)
	if err != nil {
		return nil, err
	}

	mt, err := s.merchants.GetByID(ctx, pt.MerchantID, false)
	if err != nil {
		return nil, err
	}

	result := &DetailedPayment{
		Payment:  pt,
		Merchant: mt,
	}

	if pt.CustomerID != nil {
		person, errPerson := s.payments.GetCustomerByID(ctx, merchantID, *pt.CustomerID)
		if errPerson != nil {
			return nil, errors.Wrap(errPerson, "unable to get customer")
		}
		result.Customer = person
	}

	paymentMethod, err := s.payments.GetPaymentMethod(ctx, pt)
	switch {
	case errors.Is(err, payment.ErrPaymentMethodNotSet):
		// okay, that's fine, payment method is not set by the user yet
	case err != nil:
		return nil, errors.Wrap(err, "unable to get payment method")
	case err == nil:
		result.PaymentMethod = paymentMethod
	}

	withPaymentInfo := paymentMethod != nil && !pt.IsEditable()

	if withPaymentInfo {
		tx := paymentMethod.TX()
		if tx == nil {
			return nil, errors.Wrap(ErrTransaction, "transaction is nil")
		}

		var expiresAt time.Time
		if pt.ExpiresAt != nil {
			expiresAt = *pt.ExpiresAt
		}

		paymentLink, err := tx.PaymentLink()
		if err != nil {
			return nil, err
		}

		result.PaymentInfo = &PaymentInfo{
			Status:           pt.PublicStatus(),
			PaymentLink:      paymentLink,
			RecipientAddress: tx.RecipientAddress,

			Amount:          tx.Amount.StringRaw(),
			AmountFormatted: tx.Amount.String(),

			ExpiresAt:             expiresAt,
			ExpirationDurationMin: pt.ExpirationDurationMin(),

			SuccessAction:  pt.PublicSuccessAction(),
			SuccessURL:     pt.PublicSuccessURL(),
			SuccessMessage: pt.PublicSuccessMessage(),
		}
	}

	return result, nil
}

// LockPaymentOptions locks payment editing.
// This method is used to finish payment setup by the end customer.
func (s *Service) LockPaymentOptions(ctx context.Context, merchantID, paymentID int64) error {
	details, err := s.GetDetailedPayment(ctx, merchantID, paymentID)
	if err != nil {
		return errors.Wrap(err, "unable to get detailed payment")
	}

	if !details.Payment.IsEditable() {
		return nil
	}

	if details.Customer == nil || details.PaymentMethod == nil {
		return ErrPaymentOptionsMissing
	}

	_, err = s.payments.Update(ctx, merchantID, paymentID, payment.UpdateProps{Status: payment.StatusLocked})
	if err != nil {
		return errors.Wrap(err, "unable to lock payment")
	}

	return nil
}

// SetPaymentMethod created/changes payment's underlying transaction.
func (s *Service) SetPaymentMethod(ctx context.Context, p *payment.Payment, ticker string) (*payment.Method, error) {
	if p == nil {
		return nil, errors.New("payment is nil")
	}

	if !p.IsEditable() {
		return nil, ErrStatusInvalid
	}

	mt, err := s.merchants.GetByID(ctx, p.MerchantID, false)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get merchant")
	}

	currency, err := s.getPaymentMethod(ctx, mt, ticker)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get payment method")
	}

	lockKey := lock.RowKey{Table: "payments", ID: p.ID}

	var (
		method    *payment.Method
		errReturn error
	)

	_ = s.locker.Do(ctx, lockKey, func() error {
		tx, err := s.transactions.GetLatestByPaymentID(ctx, p.ID)

		switch {
		case errors.Is(err, transaction.ErrNotFound):
			// case 1. no transaction yet -> create
			method, errReturn = s.createIncomingTransaction(ctx, p, currency)
		case err != nil:
			// case 2. unknown error
			errReturn = errors.Wrap(err, "unable to get latest payment by id")
		case tx.Status == transaction.StatusCancelled:
			// case 1*. transaction was canceled, but BE had error while changing payment method earlier.
			// This can happen when for example currency provider returns error when fetching currency rates.
			method, errReturn = s.createIncomingTransaction(ctx, p, currency)
		case tx.Currency.Ticker == currency.Ticker && tx.Currency.NetworkID == currency.NetworkID:
			// case 3. no changes, do nothing
			method = payment.MakeMethod(tx, currency)
		default:
			// case 4. ticker has changed. Change pending transaction.
			method, errReturn = s.changePaymentMethod(ctx, p, tx, currency)
		}

		return nil
	})

	return method, errReturn
}

func (s *Service) getPaymentMethod(ctx context.Context, mt *merchant.Merchant, ticker string) (money.CryptoCurrency, error) {
	currency, err := s.blockchain.GetCurrencyByTicker(ticker)
	if err != nil {
		return money.CryptoCurrency{}, errors.Wrap(err, "unable to get currency by ticker")
	}

	supported, err := s.merchants.ListSupportedCurrencies(ctx, mt)
	if err != nil {
		return money.CryptoCurrency{}, errors.Wrap(err, "unable to list merchant currencies")
	}

	for i := range supported {
		if supported[i].Currency.Ticker == currency.Ticker && supported[i].Enabled {
			return currency, nil
		}
	}

	err = errors.Wrapf(blockchain.ErrCurrencyNotFound, "currency %q is disabled for merchant", currency.Ticker)

	return money.CryptoCurrency{}, err
}

// EnsureOutboundWallet makes sure that outbound wallet for specified blockchain exists in the database
// and the system is subscribed to all of selected currencies both for mainnet & testnet.
// Returning bool indicates whether the wallet was created or returned from db.
func (s *Service) EnsureOutboundWallet(ctx context.Context, chain money.Blockchain) (*wallet.Wallet, bool, error) {
	currencies := s.blockchain.ListBlockchainCurrencies(chain)
	if len(currencies) == 0 {
		return nil, false, errors.New("currencies are empty")
	}

	// wallet should exist in DB
	w, justCreated, err := s.wallets.EnsureOutboundWallet(ctx, kmswallet.Blockchain(chain))
	if err != nil {
		return nil, false, errors.Wrap(err, "unable to ensure outbound wallet")
	}

	// wallet should be subscribed to notifications
	if err := s.ensureWalletSubscription(ctx, w, currencies[0]); err != nil {
		return nil, false, err
	}

	// wallet should have balances records, even if they're empty
	//nolint:gocritic
	for _, currency := range currencies {
		if _, err := s.wallets.EnsureBalance(ctx, wallet.EntityTypeWallet, w.ID, currency, false); err != nil {
			return nil, false, errors.Wrapf(err, "unable to ensure mainnet balance for %s", currency.Ticker)
		}

		if _, err := s.wallets.EnsureBalance(ctx, wallet.EntityTypeWallet, w.ID, currency, true); err != nil {
			return nil, false, errors.Wrapf(err, "unable to ensure testnet balance for %s", currency.Ticker)
		}
	}

	return w, justCreated, nil
}

// createIncomingTransaction creates transaction that represents pending payment created by merchant.
// Each time customer changes payment method (e.g. switching from ETH to ETH_USDT in payment UI) we need
// to create a new tx.
func (s *Service) createIncomingTransaction(
	ctx context.Context,
	pt *payment.Payment,
	currency money.CryptoCurrency,
) (*payment.Method, error) {
	// 1. Calculate service fee in crypto and USD price.
	conv, err := s.blockchain.FiatToCrypto(ctx, pt.Price, currency)
	if err != nil {
		return nil, err
	}

	cryptoAmount := conv.To

	var cryptoServiceFee money.Money
	if s.config.DefaultServiceFee > 0 {
		cryptoServiceFee, err = cryptoAmount.MultiplyFloat64(s.config.DefaultServiceFee)
		if err != nil {
			return nil, errors.Wrap(err, "unable to calculate service fee")
		}
	}

	conv, err = s.blockchain.FiatToFiat(ctx, pt.Price, money.USD)
	if err != nil {
		return nil, err
	}

	usdAmount := conv.To

	// 2. Acquire available inbound wallet or create one.
	acquiredWallet, err := s.wallets.AcquireLock(ctx, pt.MerchantID, currency, pt.IsTest)
	if err != nil {
		return nil, errors.Wrap(err, "unable to acquire wallet")
	}

	// 3. Subscribe to notifications
	if errSubs := s.ensureWalletSubscription(ctx, acquiredWallet, currency); errSubs != nil {
		return nil, errors.Wrap(errSubs, "unable to ensure wallet subscription to notifications")
	}

	// 4. Create transaction record
	tx, err := s.transactions.Create(ctx, pt.MerchantID, transaction.CreateTransaction{
		Type:            transaction.TypeIncoming,
		EntityID:        pt.ID,
		RecipientWallet: acquiredWallet,
		Currency:        currency,
		Amount:          cryptoAmount,
		ServiceFee:      cryptoServiceFee,
		USDAmount:       usdAmount,
		IsTest:          pt.IsTest,
	})

	if err != nil {
		s.logger.Err(err).
			Str("ticker", currency.Ticker).
			Int64("payment_id", pt.MerchantID).
			Msg("unable to create transaction")

		errRelease := s.wallets.ReleaseLock(
			ctx,
			acquiredWallet.ID,
			currency.Ticker,
			tx.NetworkID(),
		)

		if errRelease != nil {
			return nil, errors.Wrap(errRelease, "unable to release wallet")
		}

		return nil, errors.Wrap(err, "unable to create tx")
	}

	return payment.MakeMethod(tx, currency), nil
}

func (s *Service) changePaymentMethod(
	ctx context.Context,
	p *payment.Payment,
	tx *transaction.Transaction,
	currency money.CryptoCurrency,
) (*payment.Method, error) {
	const cancelReason = "customer chose another payment method"

	if tx.RecipientWalletID == nil {
		return nil, errors.New("wallet id is nil")
	}

	if err := s.transactions.Cancel(ctx, tx, transaction.StatusCancelled, cancelReason, nil); err != nil {
		return nil, errors.Wrap(err, "unable to mark transaction as canceled")
	}

	return s.createIncomingTransaction(ctx, p, currency)
}

func (s *Service) ensureWalletSubscription(ctx context.Context, w *wallet.Wallet, currency money.CryptoCurrency) error {
	params := func(networkID string, isTest bool) tatum.SubscriptionParams {
		return tatum.SubscriptionParams{
			Blockchain: w.Blockchain.ToMoneyBlockchain(),
			Address:    w.Address,
			WebhookURL: s.walletWebhookURL(networkID, w.UUID),
			IsTest:     isTest,
		}
	}

	var updateRecord bool

	if w.TatumSubscription.TestnetSubscriptionID == "" {
		id, err := s.tatumProvider.SubscribeToWebhook(ctx, params(currency.TestNetworkID, true))
		if err != nil {
			return errors.Wrap(err, "unable to subscribe to webhooks for testnet")
		}

		w.TatumSubscription.TestnetSubscriptionID = id
		updateRecord = true
	}

	if w.TatumSubscription.MainnetSubscriptionID == "" {
		id, err := s.tatumProvider.SubscribeToWebhook(ctx, params(currency.NetworkID, false))
		if err != nil {
			return errors.Wrap(err, "unable to subscribe to webhooks for mainnet")
		}

		w.TatumSubscription.MainnetSubscriptionID = id
		updateRecord = true
	}

	if updateRecord {
		if err := s.wallets.UpdateTatumSubscription(ctx, w, w.TatumSubscription); err != nil {
			return errors.Wrap(err, "unable to update wallet")
		}
	}

	return nil
}

func (s *Service) walletWebhookURL(networkID string, walletID uuid.UUID) string {
	return fmt.Sprintf("%s/api/webhook/v1/tatum/%s/%s", s.config.WebhookBasePath, networkID, walletID.String())
}
