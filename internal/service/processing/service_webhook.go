package processing

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	kms "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/pkg/errors"
)

// TatumWebhook see https://apidoc.tatum.io/tag/Notification-subscriptions#operation/createSubscription
type TatumWebhook struct {
	SubscriptionType string `json:"subscriptionType"`
	TransactionID    string `json:"txId"`
	Address          string `json:"address"`
	Sender           string `json:"counterAddress"`

	// Asset coin name or token contact address or (!) token ticker e.g. USDT_TRON
	Asset string `json:"asset"`

	// Amount "0.000123" (float) instead of "123" (wei-like)
	Amount      string `json:"amount"`
	BlockNumber int64  `json:"blockNumber"`

	// Type can be ['native', 'token', `trc20', 'fee'] or maybe any other
	Type string `json:"type"`

	// Mempool (EMV-based blockchains only) if appears and set to "true", the transaction is in the mempool;
	// if set to "false" or does not appear at all, the transaction has been added to a block
	Mempool bool `json:"mempool"`

	// Chain for ETH test is might be 'ethereum-goerli' or 'sepolia'
	Chain string `json:"chain"`
}

func (w *TatumWebhook) MarshalBinary() ([]byte, error) {
	return json.Marshal(w)
}

func (w *TatumWebhook) CurrencyType() money.CryptoCurrencyType {
	if w.Type == "native" {
		return money.Coin
	}

	return money.Token
}

// ValidateWebhookSignature performs HMAC signature validation
func (s *Service) ValidateWebhookSignature(body []byte, hash string) error {
	if valid := s.tatumProvider.ValidateHMAC(body, hash); !valid {
		return ErrSignatureVerification
	}

	return nil
}

func (s *Service) ProcessIncomingWebhook(ctx context.Context, walletID uuid.UUID, networkID string, wh TatumWebhook) error {
	// 0. Omit certain webhooks
	switch {
	case wh.Mempool:
		s.logger.Info().Str("blockchain_tx_hash_id", wh.TransactionID).Msg("skipping mempool transaction")
		return nil
	case wh.Type == "fee":
		s.logger.Info().Str("blockchain_tx_hash_id", wh.TransactionID).Msg("skipping fee webhook")
		return nil
	}

	// 1. Resolve wallet
	wt, err := s.wallets.GetByUUID(ctx, walletID)
	if err != nil {
		return errors.Wrap(err, "unable to get wallet by uuid")
	}

	// 2. Resolve currency
	currency, err := s.resolveCurrencyFromWebhook(wt.Blockchain.ToMoneyBlockchain(), networkID, wh)
	if err != nil {
		return errors.Wrap(err, "unable to resolve currency from webhook")
	}

	amount, err := money.CryptoFromStringFloat(currency.Ticker, wh.Amount, currency.Decimals)
	if err != nil {
		return errors.Wrap(err, "unable to make crypto amount from webhook data")
	}

	input := Input{
		Currency:      currency,
		Amount:        amount,
		SenderAddress: wh.Sender,
		TransactionID: wh.TransactionID,
		NetworkID:     networkID,
	}

	processors := []webhookProcessor{
		s.processTronAccountActivation,
		s.processExpectedWebhook,
		s.processUnexpectedWebhook,
	}

	for _, ingest := range processors {
		err := ingest(ctx, wt, input)

		// webhook was skipped by current processor. Try next one
		if errors.Is(err, errSkippedProcessor) {
			continue
		}

		// current processor failed. Return error
		if err != nil {
			return errors.Wrap(err, "unable to process webhook")
		}

		// webhook was processed successfully. Break the loop
		break
	}

	return nil
}

var errSkippedProcessor = errors.New("processor is skipped")

type webhookProcessor func(ctx context.Context, wt *wallet.Wallet, input Input) error

func (s *Service) processExpectedWebhook(ctx context.Context, wt *wallet.Wallet, input Input) error {
	tx, err := s.transactions.GetByFilter(ctx, transaction.Filter{
		RecipientWalletID: wt.ID,
		NetworkID:         input.NetworkID,
		Currency:          input.Currency.Ticker,
		Statuses:          []transaction.Status{transaction.StatusPending},
		Types:             []transaction.Type{transaction.TypeIncoming},
		HashIsEmpty:       true,
	})

	switch {
	case errors.Is(err, transaction.ErrNotFound):
		return errSkippedProcessor
	case err != nil:
		s.logger.Warn().Err(err).
			Int64("wallet_id", wt.ID).
			Str("blockchain_tx_hash_id", input.TransactionID).
			Msg("unable to find transaction")

		return errors.Wrap(err, "unable to find transaction")
	}

	if err := s.ProcessInboundTransaction(ctx, tx, wt, input); err != nil {
		return errors.Wrap(err, "unable to process incoming transaction")
	}

	s.logger.Info().
		Str("transaction_type", string(tx.Type)).
		Int64("wallet_id", wt.ID).
		Int64("transaction_id", tx.ID).
		Str("blockchain_tx_hash_id", input.TransactionID).
		Msg("Processed incoming transaction")

	return nil
}

func (s *Service) processUnexpectedWebhook(ctx context.Context, wt *wallet.Wallet, input Input) error {
	tx, err := s.transactions.GetByHash(ctx, input.NetworkID, input.TransactionID)

	switch {
	case errors.Is(err, transaction.ErrNotFound):
		if errCreate := s.createUnexpectedTransaction(ctx, wt, input); errCreate != nil {
			return errors.Wrap(errCreate, "unable to create unexpected transaction")
		}
		return nil
	case err != nil:
		return errors.Wrap(err, "unable to get transaction by hash")
	}

	s.logger.Info().
		Int64("wallet_id", wt.ID).
		Int64("transaction_id", tx.ID).
		Str("currency", input.Amount.Ticker()).
		Str("network_id", input.NetworkID).
		Msg("Skipping unexpected webhook")

	return nil
}

// https://developers.tron.network/docs/account#account-activation
func (s *Service) processTronAccountActivation(ctx context.Context, wt *wallet.Wallet, input Input) error {
	isTronCoin := wt.Blockchain == kms.TRON && input.Currency.Type == money.Coin
	isOneTrx := input.Amount.StringRaw() == "1"

	if !isTronCoin || !isOneTrx {
		return errSkippedProcessor
	}

	s.logger.Info().
		Int64("wallet_id", wt.ID).
		Str("blockchain_tx_hash_id", input.TransactionID).
		Str("currency", input.Amount.Ticker()).
		Str("network_id", input.NetworkID).
		Msg("received address activation transaction")

	return s.processUnexpectedWebhook(ctx, wt, input)
}

func (s *Service) resolveCurrencyFromWebhook(bc money.Blockchain, networkID string, wh TatumWebhook) (money.CryptoCurrency, error) {
	var (
		currency money.CryptoCurrency
		err      error
		isCoin   = wh.CurrencyType() == money.Coin
	)

	if isCoin {
		currency, err = s.blockchain.GetNativeCoin(bc)
	} else {
		currency, err = s.blockchain.GetCurrencyByBlockchainAndContract(bc, networkID, wh.Asset)
	}

	if err != nil {
		if !isCoin {
			s.logger.Warn().Err(err).
				Str("blockchain", bc.String()).
				Str("contract_address", wh.Asset).
				Str("transaction_hash", wh.TransactionID).
				Msg("unknown asset occurred")
		}

		return money.CryptoCurrency{}, err
	}

	// guard unknown network ids
	if currency.NetworkID != networkID && currency.TestNetworkID != networkID {
		return money.CryptoCurrency{}, errors.Errorf(
			"unknown %s network id %q, expected one of [%s, %s]",
			currency.Blockchain.String(), networkID, currency.NetworkID, currency.TestNetworkID,
		)
	}

	return currency, nil
}
