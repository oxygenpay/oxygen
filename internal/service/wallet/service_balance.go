package wallet

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/pkg/errors"
	"github.com/samber/lo"
	"golang.org/x/exp/slices"
)

type Balance struct {
	ID           int64
	UUID         uuid.UUID
	EntityType   EntityType
	EntityID     int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Network      string
	NetworkID    string
	CurrencyType money.CryptoCurrencyType
	Currency     string
	Amount       money.Money

	// UsdAmount eager-loaded. See Service.loadUSDBalances
	UsdAmount *money.Money
}

// Covers check if balance covers provided expenses.
func (b *Balance) Covers(expenses ...money.Money) error {
	amount := b.Amount

	var err error
	for _, expense := range expenses {
		amount, err = amount.Sub(expense)
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *Balance) Blockchain() money.Blockchain {
	return money.Blockchain(b.Network)
}

func (b *Balance) compatibleTo(a *Balance) bool {
	return b.Currency == a.Currency && b.NetworkID == a.NetworkID
}

type (
	EntityType       string
	BalanceOperation string
	MetaDataKey      string
	MetaData         map[MetaDataKey]string
)

const (
	EntityTypeMerchant EntityType = "merchant"
	EntityTypeWallet   EntityType = "wallet"
	EntityTypeSystem   EntityType = "system"

	OperationIncrement BalanceOperation = "increment"
	OperationDecrement BalanceOperation = "decrement"

	MetaOperation       MetaDataKey = "operation"
	MetaAmountRaw       MetaDataKey = "amountRaw"
	MetaAmountFormatted MetaDataKey = "amountFormatted"

	MetaTransactionID     MetaDataKey = "transactionID"
	MetaSenderWalletID    MetaDataKey = "senderWallerID"
	MetaRecipientWalletID MetaDataKey = "recipientWalletID"
)

func (s *Service) ListBalances(ctx context.Context, entityType EntityType, entityID int64, withUSD bool) ([]*Balance, error) {
	entries, err := s.store.ListBalances(ctx, repository.ListBalancesParams{
		EntityType: string(entityType),
		EntityID:   entityID,
	})

	if err != nil {
		return nil, err
	}

	balances := make([]*Balance, len(entries))
	for i := range entries {
		balance, err := entryToBalance(entries[i])
		if err != nil {
			return nil, err
		}

		balances[i] = balance
	}

	if withUSD {
		if err := s.loadUSDBalances(ctx, balances); err != nil {
			return nil, errors.Wrap(err, "unable to load USD balances")
		}
	}

	return balances, nil
}

type Balances map[EntityType][]*Balance

type ListAllBalancesOpts struct {
	WithUSD            bool
	WithSystemBalances bool
	HideEmpty          bool
}

func (s *Service) ListAllBalances(ctx context.Context, opts ListAllBalancesOpts) (Balances, error) {
	balances := make(Balances)

	// merchant balances
	res, err := s.store.ListAllBalancesByType(ctx, repository.ListAllBalancesByTypeParams{
		EntityType: string(EntityTypeMerchant),
		HideEmpty:  opts.HideEmpty,
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to list merchant balances")
	}

	merchantBalances, err := entitiesToBalances(res)
	if err != nil {
		return nil, err
	}

	balances[EntityTypeMerchant] = merchantBalances

	// wallet balances
	res, err = s.store.ListAllBalancesByType(ctx, repository.ListAllBalancesByTypeParams{
		EntityType: string(EntityTypeWallet),
		HideEmpty:  opts.HideEmpty,
	})

	if err != nil {
		return nil, errors.Wrap(err, "unable to list wallet balances")
	}

	walletBalances, err := entitiesToBalances(res)
	if err != nil {
		return nil, err
	}

	balances[EntityTypeWallet] = walletBalances

	if opts.WithSystemBalances {
		systemBalances, err := composeSystemBalances(merchantBalances, walletBalances)
		if err != nil {
			return nil, errors.Wrap(err, "unable to compose system balances")
		}

		balances[EntityTypeSystem] = systemBalances
	}

	if opts.WithUSD {
		for _, items := range balances {
			if err := s.loadUSDBalances(ctx, items); err != nil {
				return nil, errors.Wrap(err, "unable to load USD balances")
			}
		}
	}

	return balances, nil
}

// composeSystemBalances currently we have no system balances as distinct DB records,
// so we calculate them on the fly.
func composeSystemBalances(merchants, wallets []*Balance) ([]*Balance, error) {
	balancesMap := map[string]*Balance{}

	keyFunc := func(b *Balance) string {
		return fmt.Sprintf("%s/%s/%s", b.Currency, b.Network, b.NetworkID)
	}

	// add
	for _, w := range wallets {
		key := keyFunc(w)

		systemBalance, ok := balancesMap[key]
		if !ok {
			balancesMap[key] = &Balance{
				EntityType:   EntityTypeSystem,
				Network:      w.Network,
				NetworkID:    w.NetworkID,
				CurrencyType: w.CurrencyType,
				Currency:     w.Currency,
				Amount:       w.Amount,
			}
			continue
		}

		total, err := systemBalance.Amount.Add(w.Amount)
		if err != nil {
			return nil, errors.Wrap(err, "unable to add wallet's amount")
		}

		systemBalance.Amount = total
	}

	// subtract
	// system balance might be negative!
	for _, w := range merchants {
		key := keyFunc(w)
		systemBalance, ok := balancesMap[key]
		if !ok {
			fmt.Printf("%+v", balancesMap)
			return nil, errors.New("unable to find balance " + key)
		}

		total, err := systemBalance.Amount.SubNegative(w.Amount)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to subtract merchant's amount %s", key)
		}

		systemBalance.Amount = total
	}

	balances := lo.Values(balancesMap)
	slices.SortFunc(balances, func(a, b *Balance) bool { return keyFunc(a) < keyFunc(b) })

	return balances, nil
}

func entitiesToBalances(entries []repository.Balance) ([]*Balance, error) {
	balances := make([]*Balance, len(entries))

	for i := range entries {
		balance, err := entryToBalance(entries[i])
		if err != nil {
			return nil, err
		}

		balances[i] = balance
	}

	return balances, nil
}

func (s *Service) loadUSDBalances(ctx context.Context, balances []*Balance) error {
	for i, b := range balances {
		conv, err := s.blockchain.CryptoToFiat(ctx, b.Amount, money.USD)
		if err != nil {
			return err
		}

		balances[i].UsdAmount = &conv.To
	}

	return nil
}

func (s *Service) GetMerchantBalanceByUUID(ctx context.Context, merchantID int64, balanceID uuid.UUID) (*Balance, error) {
	return s.GetBalanceByUUID(ctx, EntityTypeMerchant, merchantID, balanceID)
}

func (s *Service) GetBalanceByUUID(ctx context.Context, entityType EntityType, entityID int64, balanceID uuid.UUID) (*Balance, error) {
	entry, err := s.store.GetBalanceByUUID(ctx, repository.GetBalanceByUUIDParams{
		EntityID:   entityID,
		EntityType: string(entityType),
		Uuid:       uuid.NullUUID{UUID: balanceID, Valid: true},
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrBalanceNotFound
	case err != nil:
		return nil, err
	}

	return entryToBalance(entry)
}

func (s *Service) GetBalanceByID(ctx context.Context, entityType EntityType, entityID, balanceID int64) (*Balance, error) {
	entry, err := s.store.GetBalanceByID(ctx, repository.GetBalanceByIDParams{
		EntityID:   entityID,
		EntityType: string(entityType),
		ID:         balanceID,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrBalanceNotFound
	case err != nil:
		return nil, err
	}

	return entryToBalance(entry)
}

func (s *Service) GetWalletsBalance(ctx context.Context, walletID int64, currency, networkID string) (*Balance, error) {
	b, err := s.store.GetBalanceByFilter(ctx, repository.GetBalanceByFilterParams{
		EntityID:   walletID,
		EntityType: string(EntityTypeWallet),
		Currency:   currency,
		NetworkID:  networkID,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrBalanceNotFound
	case err != nil:
		return nil, err
	}

	return entryToBalance(b)
}

func (s *Service) GetMerchantBalance(ctx context.Context, merchantID int64, currency, networkID string) (*Balance, error) {
	b, err := s.store.GetBalanceByFilter(ctx, repository.GetBalanceByFilterParams{
		EntityID:   merchantID,
		EntityType: string(EntityTypeMerchant),
		Currency:   currency,
		NetworkID:  networkID,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrBalanceNotFound
	case err != nil:
		return nil, err
	}

	return entryToBalance(b)
}

func (s *Service) EnsureBalance(
	ctx context.Context,
	entityType EntityType,
	entityID int64,
	currency money.CryptoCurrency,
	isTest bool,
) (*Balance, error) {
	entry, err := s.store.GetBalanceByFilter(ctx, repository.GetBalanceByFilterParams{
		EntityID:   entityID,
		EntityType: string(entityType),
		NetworkID:  currency.ChooseNetwork(isTest),
		Currency:   currency.Ticker,
	})

	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		entry, err = s.store.CreateBalance(ctx, repository.CreateBalanceParams{
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
			EntityID:     entityID,
			EntityType:   string(entityType),
			Uuid:         uuid.NullUUID{UUID: uuid.New(), Valid: true},
			Network:      currency.Blockchain.String(),
			NetworkID:    currency.ChooseNetwork(isTest),
			CurrencyType: string(currency.Type),
			Currency:     currency.Ticker,
			Decimals:     int32(currency.Decimals),
			Amount: pgtype.Numeric{
				Int:    new(big.Int).SetInt64(0),
				Status: pgtype.Present,
			},
		})
	}

	if err != nil {
		return nil, err
	}

	return entryToBalance(entry)
}

type UpdateBalanceByIDQuery struct {
	Operation BalanceOperation
	Amount    money.Money
	Comment   string
	MetaData  MetaData
}

func (q UpdateBalanceByIDQuery) validate() error {
	if q.Operation != OperationIncrement && q.Operation != OperationDecrement {
		return errors.New("invalid operation")
	}

	if q.Amount.IsZero() {
		return errors.New("invalid amount provided")
	}

	return nil
}

func (s *Service) UpdateBalanceByID(ctx context.Context, id int64, params UpdateBalanceByIDQuery) (*Balance, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}

	var result *Balance

	err := s.store.RunTransaction(ctx, func(ctx context.Context, q repository.Querier) error {
		balance, err := updateBalanceByID(ctx, q, id, params)
		if err != nil {
			return err
		}

		result = balance

		return nil
	})

	return result, err
}

func updateBalanceByID(ctx context.Context, q repository.Querier, id int64, params UpdateBalanceByIDQuery) (*Balance, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}

	// 1. Get & lock Balance for update
	b, err := q.GetBalanceByIDWithLock(ctx, id)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get balance by id")
	}

	balance, err := entryToBalance(b)
	if err != nil {
		return nil, errors.Wrap(err, "unable to construct balance")
	}

	if !balance.Amount.CompatibleTo(params.Amount) {
		return nil, errors.Wrap(err, "incompatible balances")
	}

	amount := repository.MoneyToNumeric(params.Amount)
	if params.Operation == OperationDecrement {
		amount = repository.MoneyToNegNumeric(params.Amount)
	}

	// 2. Increment / Decrement balance
	b, err = q.UpdateBalanceByID(ctx, repository.UpdateBalanceByIDParams{
		ID:        balance.ID,
		UpdatedAt: time.Now(),
		Amount:    amount,
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to update balance by id")
	}

	balance, err = entryToBalance(b)
	if err != nil {
		return nil, errors.Wrap(err, "unable to construct balance")
	}

	// 3. Write audit log
	err = writeAuditLog(ctx, q, balance.ID, params.Operation, params.Amount, params.Comment, params.MetaData)
	if err != nil {
		return nil, errors.Wrap(err, "unable to write balance audit log")
	}

	return balance, nil
}

type UpdateBalancesForWithdrawal struct {
	Operation     BalanceOperation
	TransactionID int64
	System        *Balance
	Merchant      *Balance
	Amount        money.Money
	ServiceFee    money.Money
	Comment       string
}

func (params UpdateBalancesForWithdrawal) validate() error {
	valid := params.System != nil &&
		params.Merchant != nil &&
		params.TransactionID > 0 &&
		params.System.compatibleTo(params.Merchant) &&
		params.System.EntityType == EntityTypeWallet &&
		params.Merchant.EntityType == EntityTypeMerchant

	if !valid {
		return errors.New("invalid input")
	}

	if !params.Amount.CompatibleTo(params.ServiceFee) {
		return money.ErrIncompatibleMoney
	}

	return nil
}

func (s *Service) UpdateBalancesForWithdrawal(ctx context.Context, params UpdateBalancesForWithdrawal) error {
	if err := params.validate(); err != nil {
		return nil
	}

	merchantAmountDelta, err := params.Amount.Add(params.ServiceFee)
	if err != nil {
		return err
	}

	metadata := MetaData{
		MetaTransactionID:  strconv.Itoa(int(params.TransactionID)),
		MetaSenderWalletID: strconv.Itoa(int(params.System.EntityID)),
	}

	return s.store.RunTransaction(ctx, func(ctx context.Context, q repository.Querier) error {
		_, err := updateBalanceByID(ctx, q, params.Merchant.ID, UpdateBalanceByIDQuery{
			Operation: params.Operation,
			Amount:    merchantAmountDelta,
			Comment:   params.Comment,
			MetaData:  metadata,
		})
		if err != nil {
			return errors.Wrap(ErrInsufficienceMerchantBalance, err.Error())
		}

		_, err = updateBalanceByID(ctx, q, params.System.ID, UpdateBalanceByIDQuery{
			Operation: params.Operation,
			Amount:    params.Amount,
			Comment:   params.Comment,
			MetaData:  metadata,
		})
		if err != nil {
			return errors.Wrap(err, "unable to update outbound wallet's balance")
		}

		return nil
	})
}

type UpdateBalanceQuery struct {
	EntityID   int64
	EntityType EntityType

	Operation BalanceOperation

	Currency money.CryptoCurrency
	Amount   money.Money

	Comment  string
	MetaData MetaData

	IsTest bool
}

func (p UpdateBalanceQuery) validate() error {
	if p.EntityID == 0 || p.EntityType == "" {
		return errors.New("invalid entity data provided")
	}

	if p.Currency.Ticker == "" {
		return errors.New("invalid currency provided")
	}

	if p.Amount.Ticker() == "" {
		return errors.New("invalid amount provided")
	}

	if p.Operation != OperationIncrement && p.Operation != OperationDecrement {
		return errors.New("invalid operation")
	}

	return nil
}

// UpdateBalance increments/decrements balance of entity (wallet / merchant) and optionally adds audit log.
// increment works based on postgres "upsert" feature: "INSERT ... ON CONFLICT (...) DO UPDATE ...".
// Balance is created if not exists.
//
// Warning: this function is meant to be used only in db transactions
func UpdateBalance(ctx context.Context, q repository.Querier, params UpdateBalanceQuery) (*Balance, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}

	amount := repository.MoneyToNumeric(params.Amount)
	if params.Operation == OperationDecrement {
		amount = repository.MoneyToNegNumeric(params.Amount)
	}

	// get balance with lock and update amount value. If not created yet, create with amount.
	balance, err := q.GetBalanceByFilterWithLock(ctx, repository.GetBalanceByFilterWithLockParams{
		EntityID:   params.EntityID,
		EntityType: string(params.EntityType),
		NetworkID:  params.Currency.ChooseNetwork(params.IsTest),
		Currency:   params.Currency.Ticker,
	})

	if err != nil && errors.Is(err, pgx.ErrNoRows) {
		balance, err = q.CreateBalance(ctx, repository.CreateBalanceParams{
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),

			EntityID:   params.EntityID,
			EntityType: string(params.EntityType),
			Uuid:       uuid.NullUUID{UUID: uuid.New(), Valid: true},

			Network:      params.Currency.Blockchain.String(),
			NetworkID:    params.Currency.ChooseNetwork(params.IsTest),
			CurrencyType: string(params.Currency.Type),
			Currency:     params.Amount.Ticker(),
			Decimals:     int32(params.Amount.Decimals()),
			Amount:       amount,
		})
	} else {
		balance, err = q.UpdateBalanceByID(ctx, repository.UpdateBalanceByIDParams{
			ID:        balance.ID,
			UpdatedAt: time.Now(),
			Amount:    amount,
		})
	}

	if err != nil {
		return nil, errors.Wrapf(err, "unable to %s balance", params.Operation)
	}

	err = writeAuditLog(ctx, q, balance.ID, params.Operation, params.Amount, params.Comment, params.MetaData)
	if err != nil {
		return nil, errors.Wrap(err, "unable to write balance audit log")
	}

	return entryToBalance(balance)
}

func writeAuditLog(
	ctx context.Context,
	q repository.Querier,
	balanceID int64,
	operation BalanceOperation,
	amount money.Money,
	comment string,
	metaData MetaData,
) error {
	if metaData == nil {
		metaData = make(MetaData)
	}

	metaData[MetaOperation] = string(operation)
	metaData[MetaAmountRaw] = amount.StringRaw()
	metaData[MetaAmountFormatted] = amount.String()

	return q.InsertBalanceAuditLog(ctx, repository.InsertBalanceAuditLogParams{
		CreatedAt: time.Now(),
		BalanceID: balanceID,
		Comment:   comment,
		Metadata:  metaData.ToJSONB(),
	})
}

func (m MetaData) ToJSONB() pgtype.JSONB {
	if len(m) == 0 {
		return pgtype.JSONB{Status: pgtype.Null}
	}

	metaRaw, _ := json.Marshal(m)

	return pgtype.JSONB{Bytes: metaRaw, Status: pgtype.Present}
}

func entryToBalance(b repository.Balance) (*Balance, error) {
	amount, err := repository.NumericToMoney(b.Amount, money.Crypto, b.Currency, int64(b.Decimals))
	if err != nil {
		return nil, err
	}

	return &Balance{
		ID:           b.ID,
		UUID:         b.Uuid.UUID,
		EntityType:   EntityType(b.EntityType),
		EntityID:     b.EntityID,
		CreatedAt:    b.CreatedAt,
		UpdatedAt:    b.UpdatedAt,
		NetworkID:    b.NetworkID,
		Network:      b.Network,
		CurrencyType: money.CryptoCurrencyType(b.CurrencyType),
		Currency:     b.Currency,
		Amount:       amount,
	}, nil
}
