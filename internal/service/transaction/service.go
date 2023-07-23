package transaction

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type Service struct {
	store      *repository.Store
	blockchain blockchain.Resolver
	wallets    *wallet.Service
	logger     *zerolog.Logger
}

const (
	// MerchantIDWildcard discards filtration by merchant id
	MerchantIDWildcard = int64(-1)

	// SystemMerchantID indicates txs related to system
	SystemMerchantID = int64(0)
)

var (
	ErrNotFound            = errors.New("transaction not found")
	ErrSameStatus          = errors.New("status not changed")
	ErrInvalidUpdateParams = errors.New("invalid update params")
)

func New(
	store *repository.Store,
	blockchainService blockchain.Resolver,
	wallets *wallet.Service,
	logger *zerolog.Logger,
) *Service {
	log := logger.With().Str("channel", "transaction_service").Logger()

	return &Service{
		store:      store,
		blockchain: blockchainService,
		wallets:    wallets,
		logger:     &log,
	}
}

type CreateTransaction struct {
	Type Type

	// EntityID that is related to that tx.
	// In case of incoming tx that's payment id
	// In case of internal tx that's 0
	// In case of withdrawal tx that's payment (withdrawal) id
	EntityID int64

	SenderAddress string
	SenderWallet  *wallet.Wallet

	RecipientAddress string
	RecipientWallet  *wallet.Wallet

	TransactionHash string

	Currency   money.CryptoCurrency
	Amount     money.Money
	USDAmount  money.Money
	ServiceFee money.Money

	IsTest bool

	isIncomingUnexpected bool
}

//nolint:gocyclo
func (c *CreateTransaction) validate() error {
	if !c.Type.valid() {
		return errors.New("invalid type")
	}

	if !c.ServiceFee.IsZero() && !c.Amount.CompatibleTo(c.ServiceFee) {
		return errors.New("service fee does not match amount")
	}

	if c.Currency.Ticker != c.Amount.Ticker() {
		return errors.New("invalid currency specified")
	}

	if c.USDAmount.Type() != money.Fiat || c.USDAmount.Ticker() != money.USD.String() {
		return errors.New("invalid usd amount")
	}

	if c.Amount.IsZero() {
		return errors.New("amount can't be zero")
	}

	switch c.Type {
	case TypeIncoming:
		if !c.isIncomingUnexpected && c.EntityID == 0 {
			return errors.New("invalid entity id")
		}

		if c.RecipientWallet == nil {
			return errors.New("empty recipient wallet")
		}

		if c.isIncomingUnexpected && c.TransactionHash == "" {
			return errors.New("empty transaction hash")
		}
	case TypeInternal:
		if c.EntityID != 0 {
			return errors.New("entity id should be 0 if tx is internal")
		}

		if c.SenderWallet == nil {
			return errors.New("empty sender wallet")
		}

		if c.RecipientWallet == nil {
			return errors.New("empty recipient wallet")
		}
	case TypeWithdrawal:
		if c.EntityID == 0 {
			return errors.New("invalid entity id")
		}

		if c.SenderWallet == nil {
			return errors.New("empty sender wallet")
		}

		if c.RecipientAddress == "" {
			return errors.New("empty recipient address")
		}
	}

	return nil
}

func (c *CreateTransaction) status() Status {
	if c.isIncomingUnexpected {
		return StatusInProgress
	}

	return StatusPending
}

type CreateOpt func(p *CreateTransaction)

// IncomingUnexpected marks that tx should be stored as StatusCompleted w/o any further processing.
func IncomingUnexpected() CreateOpt {
	return func(p *CreateTransaction) {
		if p.Type == TypeIncoming {
			p.isIncomingUnexpected = true
		}
	}
}

// Create creates transaction record in the database. If Type is TypeInternal, merchantID should be zero.
func (s *Service) Create(
	ctx context.Context,
	merchantID int64,
	params CreateTransaction,
	opts ...CreateOpt,
) (*Transaction, error) {
	for _, opt := range opts {
		opt(&params)
	}

	if err := params.validate(); err != nil {
		return nil, err
	}

	senderAddress := sql.NullString{String: params.SenderAddress, Valid: params.SenderAddress != ""}
	senderWalletID := sql.NullInt64{}
	if params.SenderWallet != nil {
		senderAddress = repository.StringToNullable(params.SenderWallet.Address)
		senderWalletID = repository.Int64ToNullable(params.SenderWallet.ID)
	}

	recipientAddress := params.RecipientAddress
	recipientWalletID := sql.NullInt64{}
	if params.RecipientWallet != nil {
		recipientAddress = params.RecipientWallet.Address
		recipientWalletID = repository.Int64ToNullable(params.RecipientWallet.ID)
	}

	networkCurrency, err := s.blockchain.GetNativeCoin(params.Currency.Blockchain)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get network currency")
	}

	factAmount := pgtype.Numeric{Status: pgtype.Null}
	metaData := pgtype.JSONB{Status: pgtype.Null}

	if params.isIncomingUnexpected {
		factAmount = repository.MoneyToNumeric(params.Amount)
		metaData = MetaData{MetaComment: "Unexpected transaction"}.toJSONB()
	}

	now := time.Now()
	tx := repository.Transaction{}

	err = s.store.RunTransaction(ctx, func(ctx context.Context, q repository.Querier) error {
		create := repository.CreateTransactionParams{
			CreatedAt: now,
			UpdatedAt: now,

			MerchantID: merchantID,
			EntityID:   sql.NullInt64{Int64: params.EntityID, Valid: params.EntityID != 0},

			Status: string(params.status()),
			Type:   string(params.Type),

			SenderWalletID: senderWalletID,
			SenderAddress:  senderAddress,

			RecipientWalletID: recipientWalletID,
			RecipientAddress:  recipientAddress,
			TransactionHash:   repository.StringToNullable(params.TransactionHash),

			Blockchain:      params.Currency.Blockchain.String(),
			NetworkID:       repository.StringToNullable(params.Currency.ChooseNetwork(params.IsTest)),
			CurrencyType:    string(params.Currency.Type),
			Currency:        params.Currency.Ticker,
			Decimals:        int32(params.Amount.Decimals()),
			NetworkDecimals: int32(networkCurrency.Decimals),

			Amount:     repository.MoneyToNumeric(params.Amount),
			FactAmount: factAmount,
			NetworkFee: pgtype.Numeric{Status: pgtype.Null},
			ServiceFee: repository.MoneyToNumeric(params.ServiceFee),
			UsdAmount:  repository.MoneyToNumeric(params.USDAmount),

			Metadata: metaData,
			IsTest:   params.IsTest,
		}

		entry, errCreate := q.CreateTransaction(ctx, create)
		if errCreate != nil {
			return errCreate
		}

		tx = entry

		return nil
	})

	if err != nil {
		return nil, err
	}

	return s.entryToTransaction(tx)
}

func (s *Service) GetByID(ctx context.Context, merchantID, id int64) (*Transaction, error) {
	return s.getByID(ctx, s.store, merchantID, id)
}

func (s *Service) GetByHash(ctx context.Context, networkID, txHash string) (*Transaction, error) {
	tx, err := s.store.GetTransactionByHashAndNetworkID(ctx, repository.GetTransactionByHashAndNetworkIDParams{
		NetworkID:       repository.StringToNullable(networkID),
		TransactionHash: repository.StringToNullable(txHash),
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return s.entryToTransaction(tx)
}

func (s *Service) getByID(ctx context.Context, q repository.Querier, merchantID, id int64) (*Transaction, error) {
	tx, err := q.GetTransactionByID(ctx, repository.GetTransactionByIDParams{
		ID:                 id,
		MerchantID:         merchantID,
		FilterByMerchantID: merchantID != MerchantIDWildcard,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return s.entryToTransaction(tx)
}

func (s *Service) GetLatestByPaymentID(ctx context.Context, paymentID int64) (*Transaction, error) {
	tx, err := s.store.GetLatestTransactionByPaymentID(ctx, repository.Int64ToNullable(paymentID))

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return s.entryToTransaction(tx)
}

func (s *Service) EagerLoadByPaymentIDs(ctx context.Context, merchantID int64, paymentIDs []int64) ([]*Transaction, error) {
	txs, err := s.store.EagerLoadTransactionsByPaymentID(ctx, repository.EagerLoadTransactionsByPaymentIDParams{
		MerchantID: merchantID,
		EntityIds:  util.MapSlice(paymentIDs, func(i int64) int32 { return int32(i) }),
		Type:       []string{string(TypeIncoming), string(TypeWithdrawal)},
	})
	if err != nil {
		return nil, err
	}

	results := make([]*Transaction, len(txs))
	for i := range txs {
		tx, err := s.entryToTransaction(txs[i])
		if err != nil {
			return nil, err
		}

		results[i] = tx
	}

	return results, nil
}

// Filter filter for resolving tx by wallet, network & currency and possible types/statuses.
// If types/statuses field is empty, then filter this type is omitted.
type Filter struct {
	RecipientWalletID int64
	NetworkID         string
	Currency          string
	Types             []Type
	Statuses          []Status
	HashIsEmpty       bool
}

func (f Filter) toRepo(limit int32) repository.GetTransactionsByFilterParams {
	return repository.GetTransactionsByFilterParams{
		FilterByRecipientWalletID: f.RecipientWalletID != 0,
		RecipientWalletID:         repository.Int64ToNullable(f.RecipientWalletID),

		FilterByNetworkID: f.NetworkID != "",
		NetworkID:         repository.StringToNullable(f.NetworkID),

		FilterByCurrency: f.Currency != "",
		Currency:         f.Currency,

		FilterByTypes: len(f.Types) > 0,
		Types:         util.ToStringMap(f.Types),

		FilterByStatuses: len(f.Statuses) > 0,
		Statuses:         util.ToStringMap(f.Statuses),

		FilterEmptyHash: f.HashIsEmpty,

		Limit: limit,
	}
}

// ListByFilter returns several txs filtered by recipient wallet id and other stuff.
func (s *Service) ListByFilter(ctx context.Context, filter Filter, limit int64) ([]*Transaction, error) {
	txs, err := s.store.GetTransactionsByFilter(ctx, filter.toRepo(int32(limit)))
	if err != nil {
		return nil, err
	}

	results := make([]*Transaction, len(txs))
	for i := range txs {
		tx, err := s.entryToTransaction(txs[i])
		if err != nil {
			return nil, err
		}

		results[i] = tx
	}

	return results, nil
}

// GetByFilter returns tx filtered by recipient wallet id and other stuff.
func (s *Service) GetByFilter(ctx context.Context, filter Filter) (*Transaction, error) {
	txs, err := s.store.GetTransactionsByFilter(ctx, filter.toRepo(1))

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	case len(txs) == 0:
		return nil, ErrNotFound
	}

	return s.entryToTransaction(txs[0])
}

func (s *Service) entryToTransaction(tx repository.Transaction) (*Transaction, error) {
	currency, err := s.blockchain.GetCurrencyByTicker(tx.Currency)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to get currency %s", tx.Currency)
	}

	amount, err := repository.NumericToMoney(tx.Amount, money.Crypto, tx.Currency, int64(tx.Decimals))
	if err != nil {
		return nil, errors.Wrap(err, "unable to construct Amount")
	}

	var factAmount *money.Money

	if tx.FactAmount.Status == pgtype.Present {
		fa, errMoney := repository.NumericToMoney(tx.FactAmount, money.Crypto, tx.Currency, int64(tx.Decimals))
		if errMoney != nil {
			return nil, errors.Wrap(errMoney, "unable to construct FactAmount")
		}

		factAmount = &fa
	}

	serviceFee, err := repository.NumericToMoney(tx.ServiceFee, money.Crypto, tx.Currency, int64(tx.Decimals))
	if err != nil {
		return nil, errors.Wrap(err, "unable to construct serviceFee")
	}

	var networkFee *money.Money
	if tx.NetworkFee.Status == pgtype.Present {
		coin, errCoin := s.blockchain.GetNativeCoin(money.Blockchain(tx.Blockchain))
		if errCoin != nil {
			return nil, errors.Wrapf(errCoin, "unable to get native coin for %q", tx.Blockchain)
		}

		netFee, errM := repository.NumericToCrypto(tx.NetworkFee, coin)
		if errM != nil {
			return nil, errors.Wrap(errM, "unable to construct networkFee")
		}

		networkFee = &netFee
	}

	usdAmount, err := repository.NumericToMoney(tx.UsdAmount, money.Fiat, money.USD.String(), 2)
	if err != nil {
		return nil, err
	}

	metaData := make(MetaData)
	if tx.Metadata.Status == pgtype.Present {
		if err := json.Unmarshal(tx.Metadata.Bytes, &metaData); err != nil {
			return nil, errors.Wrap(err, "unable to unmarshal meta data")
		}
	}

	t := &Transaction{
		ID: tx.ID,

		CreatedAt: tx.CreatedAt,
		UpdatedAt: tx.UpdatedAt,

		MerchantID: tx.MerchantID,
		EntityID:   tx.EntityID.Int64,

		Type:   Type(tx.Type),
		Status: Status(tx.Status),

		RecipientAddress:  tx.RecipientAddress,
		RecipientWalletID: repository.NullableInt64ToPointer(tx.RecipientWalletID),

		SenderAddress:  repository.NullableStringToPointer(tx.SenderAddress),
		SenderWalletID: repository.NullableInt64ToPointer(tx.SenderWalletID),

		HashID: repository.NullableStringToPointer(tx.TransactionHash),

		Currency: currency,

		Amount:     amount,
		USDAmount:  usdAmount,
		FactAmount: factAmount,
		ServiceFee: serviceFee,
		NetworkFee: networkFee,

		MetaData: metaData,
		IsTest:   tx.IsTest,
	}

	return t, nil
}
