package blockchain

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/antihax/optional"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	kms "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/provider/tatum"
	client "github.com/oxygenpay/tatum-sdk/tatum"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Broadcaster interface {
	BroadcastTransaction(ctx context.Context, blockchain money.Blockchain, hex string, isTest bool) (string, error)
	GetTransactionReceipt(ctx context.Context, blockchain money.Blockchain, transactionID string, isTest bool) (*TransactionReceipt, error)
}

func (s *Service) BroadcastTransaction(ctx context.Context, blockchain money.Blockchain, rawTX string, isTest bool) (string, error) {
	api := s.providers.Tatum.Main()
	if isTest {
		api = s.providers.Tatum.Test()
	}

	var (
		txHash client.TransactionHash
		err    error
	)

	switch kms.Blockchain(blockchain) {
	case kms.ETH:
		opts := &client.EthereumApiEthBroadcastOpts{}
		if isTest {
			opts.XTestnetType = optional.NewString(tatum.EthTestnet)
		}

		txHash, _, err = api.EthereumApi.EthBroadcast(ctx, client.BroadcastKms{TxData: rawTX}, opts)
	case kms.MATIC:
		txHash, _, err = api.PolygonApi.PolygonBroadcast(ctx, client.BroadcastKms{TxData: rawTX})
	case kms.BSC:
		txHash, _, err = api.BNBSmartChainApi.BscBroadcast(ctx, client.BroadcastKms{TxData: rawTX})
	case kms.TRON:
		hashID, errTron := s.providers.Trongrid.BroadcastTransaction(ctx, []byte(rawTX), isTest)
		if errTron != nil {
			err = errTron
		} else {
			txHash.TxId = hashID
		}
	default:
		return "", fmt.Errorf("broadcast for %q is not implemented yet", blockchain)
	}

	if err != nil {
		errSwagger, ok := err.(client.GenericSwaggerError)
		if !ok {
			return "", errors.Wrap(err, "unknown swagger error")
		}

		s.logger.Error().Err(errSwagger).
			Str("raw_tx", rawTX).
			Str("response", string(errSwagger.Body())).
			Bool("is_test", isTest).
			Msg("unable to broadcast transaction")

		return "", parseBroadcastError(blockchain, errSwagger.Body())
	}

	return txHash.TxId, nil
}

type TransactionReceipt struct {
	Blockchain    money.Blockchain
	IsTest        bool
	Sender        string
	Recipient     string
	Hash          string
	Nonce         uint64
	NetworkFee    money.Money
	Success       bool
	Confirmations int64
	IsConfirmed   bool
}

func (s *Service) GetTransactionReceipt(
	ctx context.Context,
	blockchain money.Blockchain,
	transactionID string,
	isTest bool,
) (*TransactionReceipt, error) {
	receipt, err := s.getTransactionReceipt(ctx, blockchain, transactionID, isTest)

	if err != nil {
		errSwagger, ok := err.(client.GenericSwaggerError)
		if ok {
			err = errors.Errorf(string(errSwagger.Body()))
		}

		s.logger.Error().Err(err).Msg("unable to get transaction receipt")
	}

	return receipt, err
}

func (s *Service) getTransactionReceipt(
	ctx context.Context,
	blockchain money.Blockchain,
	transactionID string,
	isTest bool,
) (*TransactionReceipt, error) {
	const (
		ethConfirmations   = 12
		maticConfirmations = 30
		bscConfirmations   = 15
	)

	nativeCoin, err := s.GetNativeCoin(blockchain)
	if err != nil {
		return nil, errors.Wrapf(err, "native coin for %q is not found", blockchain)
	}

	switch kms.Blockchain(blockchain) {
	case kms.ETH:
		rpc, err := s.providers.Tatum.EthereumRPC(ctx, isTest)
		if err != nil {
			return nil, err
		}

		return s.getEthReceipt(ctx, rpc, nativeCoin, transactionID, ethConfirmations, isTest)
	case kms.MATIC:
		rpc, err := s.providers.Tatum.MaticRPC(ctx, isTest)
		if err != nil {
			return nil, err
		}

		return s.getEthReceipt(ctx, rpc, nativeCoin, transactionID, maticConfirmations, isTest)
	case kms.BSC:
		rpc, err := s.providers.Tatum.BinanceSmartChainRPC(ctx, isTest)
		if err != nil {
			return nil, err
		}

		return s.getEthReceipt(ctx, rpc, nativeCoin, transactionID, bscConfirmations, isTest)
	case kms.TRON:
		receipt, err := s.providers.Trongrid.GetTransactionReceipt(ctx, transactionID, isTest)
		if err != nil {
			return nil, errors.Wrap(err, "unable to get tron transaction receipt")
		}

		networkFee, err := nativeCoin.MakeAmount(strconv.Itoa(int(receipt.Fee)))
		if err != nil {
			return nil, errors.Wrap(err, "unable to calculate network fee")
		}

		return &TransactionReceipt{
			Blockchain:    blockchain,
			IsTest:        isTest,
			Sender:        receipt.Sender,
			Recipient:     receipt.Recipient,
			Hash:          transactionID,
			NetworkFee:    networkFee,
			Success:       receipt.Success,
			Confirmations: receipt.Confirmations,
			IsConfirmed:   receipt.IsConfirmed,
		}, nil
	}

	return nil, kms.ErrUnknownBlockchain
}

func (s *Service) getEthReceipt(
	ctx context.Context,
	rpc *ethclient.Client,
	nativeCoin money.CryptoCurrency,
	txID string,
	requiredConfirmations int64,
	isTest bool,
) (*TransactionReceipt, error) {
	hash := common.HexToHash(txID)

	var (
		tx          *types.Transaction
		receipt     *types.Receipt
		latestBlock int64
		mu          sync.Mutex
		group       errgroup.Group
	)

	group.Go(func() error {
		txByHash, _, err := rpc.TransactionByHash(ctx, hash)
		if err != nil {
			return err
		}

		mu.Lock()
		tx = txByHash
		mu.Unlock()

		return nil
	})

	group.Go(func() error {
		r, err := rpc.TransactionReceipt(ctx, hash)
		if err != nil {
			return err
		}

		mu.Lock()
		receipt = r
		mu.Unlock()

		return nil
	})

	group.Go(func() error {
		num, err := rpc.BlockNumber(ctx)
		if err != nil {
			return err
		}

		mu.Lock()
		latestBlock = int64(num)
		mu.Unlock()

		return nil
	})

	if err := group.Wait(); err != nil {
		return nil, err
	}

	gasPrice, err := nativeCoin.MakeAmountFromBigInt(receipt.EffectiveGasPrice)
	if err != nil {
		return nil, errors.Wrap(err, "unable to construct network fee")
	}

	networkFee, err := gasPrice.MultiplyInt64(int64(receipt.GasUsed))
	if err != nil {
		return nil, errors.Wrap(err, "unable to calculate network fee")
	}

	sender, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get sender")
	}

	confirmations := latestBlock - receipt.BlockNumber.Int64()

	return &TransactionReceipt{
		Blockchain:    nativeCoin.Blockchain,
		IsTest:        isTest,
		Sender:        sender.String(),
		Recipient:     tx.To().String(),
		Hash:          txID,
		Nonce:         tx.Nonce(),
		NetworkFee:    networkFee,
		Success:       receipt.Status == 1,
		Confirmations: confirmations,
		IsConfirmed:   confirmations >= requiredConfirmations,
	}, nil
}

func parseBroadcastError(_ money.Blockchain, body []byte) error {
	// Sample response:
	//{
	//	"statusCode": 403,
	//	"errorCode": "eth.broadcast.failed",
	//	"message": "Unable to broadcast transaction.",
	//	"cause": "insufficient funds for gas * price + value [-32000]"
	//}
	type tatumErrObj struct {
		Message string `json:"message"`
		Cause   string `json:"cause"`
	}

	msg := &tatumErrObj{}
	_ = json.Unmarshal(body, msg)

	switch {
	case strings.Contains(msg.Message, "insufficient funds"):
		return ErrInsufficientFunds
	case strings.Contains(msg.Cause, "insufficient funds"):
		return ErrInsufficientFunds
	default:
		return errors.Wrap(ErrInvalidTransaction, msg.Message)
	}
}
