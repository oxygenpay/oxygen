package test

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/auth"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	kmswallet "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/user"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/samber/lo"
	"github.com/stretchr/testify/require"
)

type Must struct {
	tc *IntegrationTest
}

func (m *Must) CreateSampleUser(t *testing.T) (*user.User, string) {
	s := util.Strings.Random(8)

	return m.CreateUser(t, auth.GoogleUser{
		Name:          "user-" + s,
		Email:         fmt.Sprintf("user-%s@gmail.com", s),
		EmailVerified: true,
		Locale:        "en",
	})
}

// CreateUser creates user with api token.
func (m *Must) CreateUser(t *testing.T, params auth.GoogleUser) (*user.User, string) {
	if params.Sub == "" {
		params.Sub = util.Strings.Random(8)
	}

	person, err := m.tc.Services.Users.ResolveWithGoogle(m.tc.Context, &params)
	require.NoError(t, err)

	return person, m.CreateUserToken(t, person)
}

// CreateUserViaEmail creates user with email auth and api token.
func (m *Must) CreateUserViaEmail(t *testing.T, email, pass string) (*user.User, string) {
	person, err := m.tc.Services.Users.Register(m.tc.Context, email, pass)
	require.NoError(t, err)

	return person, m.CreateUserToken(t, person)
}

func (m *Must) CreateUserToken(t *testing.T, u *user.User) string {
	token, err := m.tc.Services.AuthTokenManager.CreateUserToken(m.tc.Context, u.ID, "test")
	require.NoError(t, err)

	return token.Token
}

// CreateMerchant creates merchant with api token.
func (m *Must) CreateMerchant(t *testing.T, userID int64) (*merchant.Merchant, string) {
	mt, err := m.tc.Services.Merchants.Create(m.tc.Context, userID, "my-store", "https://site.com", nil)
	require.NoError(t, err)

	return mt, m.CreateMerchantToken(t, mt)
}

func (m *Must) CreateMerchantToken(t *testing.T, mt *merchant.Merchant) string {
	token, err := m.tc.Services.AuthTokenManager.CreateMerchantToken(m.tc.Context, mt.ID, "test")
	require.NoError(t, err)

	return token.Token
}

func (m *Must) CreateWallet(t *testing.T, blockchain, address, pubKey string, walletType wallet.Type) *wallet.Wallet {
	m.tc.SetupCreateWalletWithSubscription(blockchain, address, pubKey)

	wt, err := m.tc.Services.Wallet.Create(m.tc.Context, kmswallet.Blockchain(blockchain), walletType)
	require.NoError(t, err)

	return wt
}

func (m *Must) CreateWalletWithBalance(
	t *testing.T,
	blockchain string,
	walletType wallet.Type,
	balanceOpts ...func(params *repository.CreateBalanceParams),
) (*wallet.Wallet, *wallet.Balance) {
	address := fmt.Sprintf("0x-fake-address-%s", util.Strings.Random(6))
	pubKey := fmt.Sprintf("0x-fake-pubkey-%s", util.Strings.Random(6))

	w := m.CreateWallet(t, blockchain, address, pubKey, walletType)
	b := m.CreateBalance(t, wallet.EntityTypeWallet, w.ID, balanceOpts...)

	return w, b
}

func (m *Must) CreateBalance(
	t *testing.T,
	entityType wallet.EntityType,
	entityID int64,
	opts ...func(params *repository.CreateBalanceParams),
) *wallet.Balance {
	ctx := m.tc.Context

	params := repository.CreateBalanceParams{
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Uuid:       uuid.NullUUID{UUID: uuid.New(), Valid: true},
		EntityID:   entityID,
		EntityType: string(entityType),
	}

	for _, opt := range opts {
		opt(&params)
	}

	record, err := m.tc.Repository.CreateBalance(ctx, params)
	require.NoError(t, err)

	b, err := m.tc.Services.Wallet.GetBalanceByUUID(ctx, entityType, entityID, record.Uuid.UUID)
	require.NoError(t, err)

	return b
}

func WithBalanceFromCurrency(currency money.CryptoCurrency, valueRaw string, isTest bool) func(params *repository.CreateBalanceParams) {
	return func(p *repository.CreateBalanceParams) {
		p.Network = currency.Blockchain.String()
		p.NetworkID = currency.ChooseNetwork(isTest)
		p.CurrencyType = string(currency.Type)
		p.Currency = currency.Ticker
		p.Decimals = int32(currency.Decimals)
		p.Amount = repository.MoneyToNumeric(lo.Must(currency.MakeAmount(valueRaw)))
	}
}

func (m *Must) CreateTransaction(t *testing.T, merchantID int64, customize ...func(params *transaction.CreateTransaction)) *transaction.Transaction {
	const ticker = "ETH_USDT"

	usd, err := money.FiatFromFloat64(money.USD, 50)
	require.NoError(t, err)

	curr, err := m.tc.Services.Blockchain.GetCurrencyByTicker(ticker)
	require.NoError(t, err)

	amount, err := money.CryptoFromRaw(ticker, "1_000_000", 6)
	require.NoError(t, err)

	fee, err := money.CryptoFromRaw(ticker, "10_000", 6)
	require.NoError(t, err)

	params := transaction.CreateTransaction{
		Type:             transaction.TypeIncoming,
		EntityID:         1, // "fake" payment id
		RecipientWallet:  nil,
		RecipientAddress: "0x123",
		Currency:         curr,
		Amount:           amount,
		USDAmount:        usd,
		ServiceFee:       fee,
		IsTest:           false,
	}

	for _, fn := range customize {
		fn(&params)
	}

	tx, err := m.tc.Services.Transaction.Create(m.tc.Context, merchantID, params)
	require.NoError(t, err)

	return tx
}

func (m *Must) CreateCustomer(t *testing.T, merchantID int64, email string) *payment.Customer {
	id := uuid.New()

	_, err := m.tc.Repository.CreateCustomer(m.tc.Context, repository.CreateCustomerParams{
		Uuid:       id,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Email:      repository.StringToNullable(email),
		MerchantID: merchantID,
	})
	require.NoError(t, err)

	c, err := m.tc.Services.Payment.GetCustomerByUUID(m.tc.Context, merchantID, id)
	require.NoError(t, err)

	return c
}

func (m *Must) GetCurrency(t *testing.T, ticker string) money.CryptoCurrency {
	c, err := m.tc.Services.Blockchain.GetCurrencyByTicker(ticker)
	require.NoError(t, err)

	return c
}

func (m *Must) GetBlockchainCoin(t *testing.T, chain money.Blockchain) money.CryptoCurrency {
	c, err := m.tc.Services.Blockchain.GetNativeCoin(chain)
	require.NoError(t, err)

	return c
}
