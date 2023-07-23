package wallet

import (
	"context"
	"encoding/json"
	"strconv"

	kms "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/util"
	kmsclient "github.com/oxygenpay/oxygen/pkg/api-kms/v1/client/wallet"
	kmsmodel "github.com/oxygenpay/oxygen/pkg/api-kms/v1/model"
	"github.com/pkg/errors"
)

func (s *Service) CreateSignedTransaction(
	ctx context.Context,
	sender *Wallet,
	recipient string,
	currency money.CryptoCurrency,
	amount money.Money,
	fee blockchain.Fee,
	isTest bool,
) (string, error) {
	nonce, err := s.IncrementPendingTransaction(ctx, sender.ID, isTest)
	if err != nil {
		return "", errors.Wrap(err, "unable to increment pending transactions counter")
	}

	txRaw, errCreate := s.createSignedTransaction(
		ctx,
		sender,
		recipient,
		currency,
		amount,
		fee,
		int64(nonce),
		isTest,
	)

	if errCreate != nil {
		if err := s.DecrementPendingTransaction(ctx, sender.ID, isTest); err != nil {
			return "", errors.Wrap(err, "unable to decrement pending transactions counter")
		}
	}

	return txRaw, errCreate
}

//nolint:gocyclo
func (s *Service) createSignedTransaction(
	ctx context.Context,
	sender *Wallet,
	recipient string,
	currency money.CryptoCurrency,
	amount money.Money,
	fee blockchain.Fee,
	nonce int64,
	isTest bool,
) (string, error) {
	if currency.Blockchain == kms.ETH.ToMoneyBlockchain() {
		networkID, err := strconv.Atoi(currency.ChooseNetwork(isTest))
		if err != nil {
			return "", errors.Wrap(err, "unable to parse network id")
		}

		ethFee, err := fee.ToEthFee()
		if err != nil {
			return "", errors.Wrap(err, "fee is not ETH")
		}

		res, err := s.kms.CreateEthereumTransaction(&kmsclient.CreateEthereumTransactionParams{
			Context:  ctx,
			WalletID: sender.UUID.String(),
			Data: &kmsmodel.CreateEthereumTransactionRequest{
				Amount:            amount.StringRaw(),
				AssetType:         kmsmodel.AssetType(currency.Type),
				ContractAddress:   currency.ChooseContractAddress(isTest),
				Gas:               int64(ethFee.GasUnits),
				MaxFeePerGas:      ethFee.GasPrice,
				MaxPriorityPerGas: ethFee.PriorityFee,
				NetworkID:         int64(networkID),
				Nonce:             util.Ptr(nonce),
				Recipient:         recipient,
			},
		})

		if err != nil {
			return "", errors.Wrap(err, "unable to create ETH transaction")
		}

		return res.Payload.RawTransaction, nil
	}

	if currency.Blockchain == kms.MATIC.ToMoneyBlockchain() {
		networkID, err := strconv.Atoi(currency.ChooseNetwork(isTest))
		if err != nil {
			return "", errors.Wrap(err, "unable to parse network id")
		}

		maticFee, err := fee.ToMaticFee()
		if err != nil {
			return "", errors.Wrap(err, "fee is not MATIC")
		}

		res, err := s.kms.CreateMaticTransaction(&kmsclient.CreateMaticTransactionParams{
			Context:  ctx,
			WalletID: sender.UUID.String(),
			Data: &kmsmodel.CreateMaticTransactionRequest{
				Amount:            amount.StringRaw(),
				AssetType:         kmsmodel.AssetType(currency.Type),
				ContractAddress:   currency.ChooseContractAddress(isTest),
				Gas:               int64(maticFee.GasUnits),
				MaxFeePerGas:      maticFee.GasPrice,
				MaxPriorityPerGas: maticFee.PriorityFee,
				NetworkID:         int64(networkID),
				Nonce:             util.Ptr(nonce),
				Recipient:         recipient,
			},
		})

		if err != nil {
			return "", errors.Wrap(err, "unable to create MATIC transaction")
		}

		return res.Payload.RawTransaction, nil
	}

	if currency.Blockchain == kms.BSC.ToMoneyBlockchain() {
		networkID, err := strconv.Atoi(currency.ChooseNetwork(isTest))
		if err != nil {
			return "", errors.Wrap(err, "unable to parse network id")
		}

		bscFee, err := fee.ToBSCFee()
		if err != nil {
			return "", errors.Wrap(err, "fee is not BSC")
		}

		res, err := s.kms.CreateBSCTransaction(&kmsclient.CreateBSCTransactionParams{
			Context:  ctx,
			WalletID: sender.UUID.String(),
			Data: &kmsmodel.CreateBSCTransactionRequest{
				Amount:            amount.StringRaw(),
				AssetType:         kmsmodel.AssetType(currency.Type),
				ContractAddress:   currency.ChooseContractAddress(isTest),
				Gas:               int64(bscFee.GasUnits),
				MaxFeePerGas:      bscFee.GasPrice,
				MaxPriorityPerGas: bscFee.PriorityFee,
				NetworkID:         int64(networkID),
				Nonce:             util.Ptr(nonce),
				Recipient:         recipient,
			},
		})

		if err != nil {
			return "", errors.Wrap(err, "unable to create BSC transaction")
		}

		return res.Payload.RawTransaction, nil
	}

	if currency.Blockchain == kms.TRON.ToMoneyBlockchain() {
		tronFee, err := fee.ToTronFee()
		if err != nil {
			return "", errors.Wrap(err, "fee is not TRON")
		}

		res, err := s.kms.CreateTronTransaction(&kmsclient.CreateTronTransactionParams{
			Context:  ctx,
			WalletID: sender.UUID.String(),
			Data: &kmsmodel.CreateTronTransactionRequest{
				Amount:          amount.StringRaw(),
				AssetType:       kmsmodel.AssetType(currency.Type),
				ContractAddress: currency.ChooseContractAddress(isTest),
				FeeLimit:        int64(tronFee.FeeLimitSun),
				IsTest:          isTest,
				Recipient:       recipient,
			},
		})

		if err != nil {
			return "", errors.Wrap(err, "unable to create TRON transaction")
		}

		resAsBytes, err := json.Marshal(res.Payload)
		if err != nil {
			return "", errors.Wrap(err, "unable to marshal TRON transaction")
		}

		return string(resAsBytes), nil
	}

	return "", errors.New("unsupported currency " + currency.Ticker)
}
