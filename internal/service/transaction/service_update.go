package transaction

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgtype"
	pgx "github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/pkg/errors"
)

type ReceiveTransaction struct {
	Status          Status
	SenderAddress   string
	TransactionHash string
	FactAmount      money.Money
	MetaData        MetaData
}

func (u ReceiveTransaction) validate() error {
	if u.Status != StatusInProgress && u.Status != StatusInProgressInvalid {
		return errors.Wrapf(ErrInvalidUpdateParams, "unsupported update status %q", u.Status)
	}

	if u.SenderAddress == "" {
		return errors.Wrap(ErrInvalidUpdateParams, "senderAddress is empty")
	}

	if u.TransactionHash == "" {
		return errors.Wrap(ErrInvalidUpdateParams, "transactionHash is empty")
	}

	if u.FactAmount.IsZero() {
		return errors.Wrap(ErrInvalidUpdateParams, "factAmount is empty")
	}

	return nil
}

// Receive updates tx when system notices tx on the blockchain, but it's not confirmed yet.
func (s *Service) Receive(
	ctx context.Context,
	merchantID, txID int64,
	params ReceiveTransaction,
) (*Transaction, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}

	var result *Transaction

	errCommit := s.store.RunTransaction(ctx, func(ctx context.Context, q repository.Querier) error {
		tx, err := s.receive(ctx, q, merchantID, txID, params)
		if err != nil {
			return err
		}

		result = tx

		if result.RecipientWalletID == nil {
			return errors.New("recipient id is nil")
		}

		errRelease := wallet.ReleaseLock(ctx, q, *tx.RecipientWalletID, tx.Currency.Ticker, tx.NetworkID())
		if errRelease != nil {
			return errors.Wrap(err, "unable to release lock")
		}

		return nil
	})

	if errCommit != nil {
		return nil, errCommit
	}

	return result, nil
}

// confirm mark tx as confirmed and updates related balances.
func (s *Service) receive(ctx context.Context, q repository.Querier, merchantID, txID int64, params ReceiveTransaction) (*Transaction, error) {
	// 1. Get transaction
	tx, err := s.getByID(ctx, q, merchantID, txID)
	if err != nil {
		return nil, err
	}

	metaData := tx.MetaData
	for k, v := range params.MetaData {
		metaData[k] = v
	}

	// 2. Confirm transaction
	entry, err := q.UpdateTransaction(ctx, repository.UpdateTransactionParams{
		MerchantID:      merchantID,
		ID:              txID,
		Status:          string(params.Status),
		UpdatedAt:       time.Now(),
		SenderAddress:   repository.StringToNullable(params.SenderAddress),
		FactAmount:      repository.MoneyToNumeric(params.FactAmount),
		NetworkFee:      pgtype.Numeric{Status: pgtype.Null},
		TransactionHash: repository.StringToNullable(params.TransactionHash),
		Metadata:        metaData.toJSONB(),
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	tx, err = s.entryToTransaction(entry)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

type ConfirmTransaction struct {
	Status          Status
	SenderAddress   string
	TransactionHash string
	FactAmount      money.Money
	NetworkFee      money.Money
	MetaData        MetaData

	allowZeroNetworkFee bool
}

func (c *ConfirmTransaction) AllowZeroNetworkFee() {
	c.allowZeroNetworkFee = true
}

func (c *ConfirmTransaction) validate() error {
	if c.Status != StatusCompleted && c.Status != StatusCompletedInvalid {
		return errors.Wrapf(ErrInvalidUpdateParams, "unsupported update status %q", c.Status)
	}

	if c.SenderAddress == "" {
		return errors.Wrap(ErrInvalidUpdateParams, "senderAddress is empty")
	}

	if c.TransactionHash == "" {
		return errors.Wrap(ErrInvalidUpdateParams, "transactionHash is empty")
	}

	if c.FactAmount.IsZero() {
		return errors.Wrap(ErrInvalidUpdateParams, "factAmount is empty")
	}

	if c.NetworkFee.IsZero() && !c.allowZeroNetworkFee {
		return errors.Wrap(ErrInvalidUpdateParams, "networkFee is empty")
	}

	return nil
}

func (s *Service) Confirm(
	ctx context.Context,
	merchantID, txID int64,
	params ConfirmTransaction,
) (*Transaction, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}

	var result *Transaction

	errCommit := s.store.RunTransaction(ctx, func(ctx context.Context, q repository.Querier) error {
		tx, err := s.confirm(ctx, q, merchantID, txID, params)
		if err != nil {
			return err
		}

		result = tx

		return nil
	})

	if errCommit != nil {
		return nil, errCommit
	}

	return result, nil
}

func (s *Service) UpdateTransactionHash(ctx context.Context, merchantID, txID int64, txHash string) error {
	return s.store.SetTransactionHash(ctx, repository.SetTransactionHashParams{
		ID:              txID,
		MerchantID:      merchantID,
		UpdatedAt:       time.Now(),
		TransactionHash: repository.StringToNullable(txHash),
	})
}

// confirm mark tx as confirmed and updates related balances.
func (s *Service) confirm(ctx context.Context, q repository.Querier, merchantID, txID int64, params ConfirmTransaction) (*Transaction, error) {
	// 1. Get transaction
	tx, err := s.getByID(ctx, q, merchantID, txID)
	if err != nil {
		return nil, err
	}

	if tx.Status == params.Status {
		return nil, ErrSameStatus
	}

	metaData := tx.MetaData
	for k, v := range params.MetaData {
		metaData[k] = v
	}

	// 2. Confirm transaction
	entry, err := q.UpdateTransaction(ctx, repository.UpdateTransactionParams{
		MerchantID:       merchantID,
		ID:               txID,
		Status:           string(params.Status),
		UpdatedAt:        time.Now(),
		SenderAddress:    repository.StringToNullable(params.SenderAddress),
		FactAmount:       repository.MoneyToNumeric(params.FactAmount),
		NetworkFee:       repository.MoneyToNumeric(params.NetworkFee),
		RemoveServiceFee: params.Status == StatusCompletedInvalid,
		TransactionHash:  repository.StringToNullable(params.TransactionHash),
		Metadata:         metaData.toJSONB(),
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	tx, err = s.entryToTransaction(entry)
	if err != nil {
		return nil, err
	}

	// 3. Update balances after transaction confirmation
	if err := s.updateBalancesAfterTxConfirmation(ctx, q, tx, params); err != nil {
		return nil, errors.Wrap(err, "unable to update balances")
	}

	return tx, nil
}

//nolint:gocyclo
func (s *Service) updateBalancesAfterTxConfirmation(
	ctx context.Context,
	q repository.Querier,
	tx *Transaction,
	params ConfirmTransaction,
) error {
	switch {
	case tx.FactAmount == nil:
		return errors.New("factAmount is nil")
	case tx.FactAmount.IsZero():
		return errors.New("factAmount is zero")
	case tx.FactAmount.IsNegative():
		return errors.New("factAmount is negative")
	}

	incrementRecipientBalance := func(ctx context.Context) (string, MetaData, error) {
		if tx.RecipientWalletID == nil {
			return "", nil, errors.New("recipient walletID is nil")
		}

		comment := fmt.Sprintf("incoming tx %s", params.TransactionHash)
		metaData := MetaData{
			MetaRecipientWalletID: strconv.FormatInt(*tx.RecipientWalletID, 10),
			MetaMerchantID:        strconv.FormatInt(tx.MerchantID, 10),
			MetaTransactionID:     strconv.FormatInt(tx.ID, 10),
		}

		updateQuery := wallet.UpdateBalanceQuery{
			EntityID:   *tx.RecipientWalletID,
			EntityType: wallet.EntityTypeWallet,

			Operation: wallet.OperationIncrement,

			Currency: tx.Currency,
			Amount:   *tx.FactAmount,

			Comment:  comment,
			MetaData: wallet.MetaData(metaData),
			IsTest:   tx.IsTest,
		}

		if _, err := wallet.UpdateBalance(ctx, q, updateQuery); err != nil {
			return "", nil, errors.Wrap(err, "unable to update wallet balance")
		}

		return comment, metaData, nil
	}

	networkCurrency := func() (money.CryptoCurrency, error) {
		currency := tx.Currency
		if tx.Currency.Type == money.Token {
			cur, err := s.blockchain.GetNativeCoin(tx.Currency.Blockchain)
			if err != nil {
				return money.CryptoCurrency{}, errors.Wrap(err, "unable to get currency for fees")
			}

			currency = cur
		}

		return currency, nil
	}

	if tx.Type == TypeIncoming {
		// Increment wallet's balance
		comment, metaData, err := incrementRecipientBalance(ctx)
		if err != nil {
			return err
		}

		// In that case there was an "underpayment".
		// So, skip merchant's balance increment
		if tx.Status == StatusCompletedInvalid {
			return nil
		}

		// If tx is unexpected, skip merchant's balance increment
		if tx.MerchantID == SystemMerchantID {
			return nil
		}

		// If customer paid more than required, restrict
		// merchant's balance increment by initial tx amount
		gainedAmount := *tx.FactAmount
		if tx.FactAmount.GreaterThan(tx.Amount) {
			gainedAmount = tx.Amount
		}

		gainedAmountMinusFee, err := gainedAmount.Sub(tx.ServiceFee)
		if err != nil {
			return errors.Wrap(err, "unable to subtract serviceFee")
		}

		updateMerchantBalance := wallet.UpdateBalanceQuery{
			EntityID:   tx.MerchantID,
			EntityType: wallet.EntityTypeMerchant,

			Currency: tx.Currency,
			Amount:   gainedAmountMinusFee,

			Operation: wallet.OperationIncrement,

			Comment:  comment,
			MetaData: wallet.MetaData(metaData),
			IsTest:   tx.IsTest,
		}

		if _, errIncrement := wallet.UpdateBalance(ctx, q, updateMerchantBalance); errIncrement != nil {
			return errors.Wrap(errIncrement, "unable to update merchant balance")
		}

		return nil
	}

	if tx.Type == TypeInternal {
		if tx.SenderWalletID == nil {
			return errors.New("sender wallet id is nil")
		}

		// Increment wallet's balance
		_, metaData, err := incrementRecipientBalance(ctx)
		if err != nil {
			return err
		}

		currency, err := networkCurrency()
		if err != nil {
			return err
		}

		// Decrement networkFee from sender's wallet COIN balance
		updateBalance := wallet.UpdateBalanceQuery{
			EntityID:   *tx.SenderWalletID,
			EntityType: wallet.EntityTypeWallet,
			Operation:  wallet.OperationDecrement,
			Currency:   currency,
			Amount:     params.NetworkFee,
			IsTest:     tx.IsTest,
			MetaData:   wallet.MetaData(metaData),
			Comment: fmt.Sprintf(
				"decrementing balance as a fee to internal tx %s (%s)",
				params.TransactionHash,
				tx.Currency.Ticker,
			),
		}

		if _, errDecrement := wallet.UpdateBalance(ctx, q, updateBalance); errDecrement != nil {
			return errors.Wrap(errDecrement, "unable to decrement balance by network fee")
		}

		return nil
	}

	if tx.Type == TypeWithdrawal {
		if tx.SenderWalletID == nil {
			return errors.New("sender wallet id is nil")
		}

		currency, err := networkCurrency()
		if err != nil {
			return err
		}

		// Decrement networkFee from sender's wallet COIN balance
		updateBalance := wallet.UpdateBalanceQuery{
			EntityID:   *tx.SenderWalletID,
			EntityType: wallet.EntityTypeWallet,
			Operation:  wallet.OperationDecrement,
			Currency:   currency,
			Amount:     params.NetworkFee,
			IsTest:     tx.IsTest,
			MetaData: wallet.MetaData{
				MetaMerchantID:    strconv.FormatInt(tx.MerchantID, 10),
				MetaTransactionID: strconv.FormatInt(tx.ID, 10),
			},
			Comment: fmt.Sprintf(
				"decrementing balance as a fee to withdrawal tx %s (%s)",
				params.TransactionHash,
				tx.Currency.Ticker,
			),
		}

		if _, errDecrement := wallet.UpdateBalance(ctx, q, updateBalance); errDecrement != nil {
			return errors.Wrap(errDecrement, "unable to decrement balance by network fee")
		}

		return nil
	}

	if tx.Type == TypeVirtual {
		// In that case there was an "underpayment".
		// So, skip merchant's balance increment
		if tx.Status != StatusCompleted || tx.MerchantID == 0 {
			return errors.New("invalid tx data for type=virtual")
		}

		comment := "virtual system topup"
		meta := wallet.MetaData{
			MetaMerchantID:    strconv.FormatInt(tx.MerchantID, 10),
			MetaTransactionID: strconv.FormatInt(tx.ID, 10),
		}

		updateMerchantBalance := wallet.UpdateBalanceQuery{
			EntityID:   tx.MerchantID,
			EntityType: wallet.EntityTypeMerchant,

			Currency: tx.Currency,
			Amount:   *tx.FactAmount,

			Operation: wallet.OperationIncrement,

			Comment:  comment,
			MetaData: meta,
			IsTest:   tx.IsTest,
		}

		if _, errIncrement := wallet.UpdateBalance(ctx, q, updateMerchantBalance); errIncrement != nil {
			return errors.Wrap(errIncrement, "unable to update merchant balance")
		}

		return nil
	}

	return fmt.Errorf("unknown transaction type %q", tx.Type)
}

func (s *Service) Cancel(ctx context.Context, tx *Transaction, status Status, reason string, setNetworkFee *money.Money) error {
	networkFee := pgtype.Numeric{Status: pgtype.Null}

	// some chains (e.g.) may have 0 network fees
	if setNetworkFee != nil && !setNetworkFee.IsZero() {
		coinBalance, err := s.wallets.GetWalletsBalance(
			ctx,
			*tx.SenderWalletID,
			setNetworkFee.Ticker(),
			tx.NetworkID(),
		)
		if err != nil {
			return errors.Wrap(err, "unable to get wallet's coin balance")
		}

		_, err = s.wallets.UpdateBalanceByID(ctx, coinBalance.ID, wallet.UpdateBalanceByIDQuery{
			Operation: wallet.OperationDecrement,
			Amount:    *setNetworkFee,
			Comment:   "network fee for canceled withdrawal",
			MetaData: wallet.MetaData{
				wallet.MetaTransactionID: strconv.Itoa(int(tx.ID)),
			},
		})
		if err != nil {
			return errors.Wrap(err, "unable to decrement network fee from wallet balance")
		}

		networkFee = repository.MoneyToNumeric(*setNetworkFee)
	}

	return s.store.RunTransaction(ctx, func(ctx context.Context, q repository.Querier) error {
		errCancel := s.store.CancelTransaction(ctx, repository.CancelTransactionParams{
			ID:            tx.ID,
			Status:        string(status),
			UpdatedAt:     time.Now(),
			Metadata:      MetaData{MetaComment: reason}.toJSONB(),
			SetNetworkFee: setNetworkFee != nil,
			NetworkFee:    networkFee,
		})
		if errCancel != nil {
			return errors.Wrap(errCancel, "unable to cancel transaction")
		}

		if tx.Type == TypeIncoming && tx.RecipientWalletID != nil {
			errRelease := wallet.ReleaseLock(ctx, q, *tx.RecipientWalletID, tx.Currency.Ticker, tx.NetworkID())
			if errRelease != nil {
				return errors.Wrap(errCancel, "unable to release wallet lock")
			}
		}

		return nil
	})
}
