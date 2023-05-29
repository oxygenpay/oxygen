package wallet

import (
	"crypto/ecdsa"
	"io"
	"math/big"
	"regexp"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/crypto/sha3"
)

// EthProvider generates wallets for eth-like chains (eth, tron, bsc, matic, ...)
type EthProvider struct {
	Blockchain   Blockchain
	CryptoReader io.Reader
}

// ethAddressRegex see https://goethereumbook.org/en/address-check/
var ethAddressRegex = regexp.MustCompile("^0x[0-9a-fA-F]{40}$")

const transferFnSignature = "transfer(address,uint256)"

func (p *EthProvider) Generate() *Wallet {
	key, err := ecdsa.GenerateKey(crypto.S256(), p.CryptoReader)
	if err != nil {
		return &Wallet{}
	}

	privateKey := hexutil.Encode(crypto.FromECDSA(key))
	publicKey := hexutil.Encode(crypto.FromECDSAPub(&key.PublicKey))
	address := crypto.PubkeyToAddress(key.PublicKey).Hex()

	return &Wallet{
		UUID:       uuid.New(),
		CreatedAt:  time.Now(),
		Blockchain: p.Blockchain,
		Address:    address,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}
}

func (p *EthProvider) GetBlockchain() Blockchain {
	return p.Blockchain
}

type EthTransactionParams struct {
	Type AssetType

	Recipient       string
	Amount          string
	ContractAddress string

	NetworkID int64
	Nonce     int64

	MaxPriorityFeePerGas string
	MaxFeePerGas         string
	Gas                  int64
}

func (p EthTransactionParams) validate() error {
	if !p.Type.Valid() {
		return errors.New("type is invalid")
	}

	if !validateEthereumAddress(p.Recipient) {
		return ErrInvalidAddress
	}

	if p.Type == Token {
		if !validateEthereumAddress(p.ContractAddress) {
			return ErrInvalidContractAddress
		}
	}

	if p.Amount == "" {
		return ErrInvalidAmount
	}

	if p.Gas == 0 || p.MaxFeePerGas == "" || p.MaxPriorityFeePerGas == "" {
		return ErrInvalidGasSettings
	}

	if p.NetworkID < 1 {
		return ErrInvalidNetwork
	}

	if p.Nonce < 0 {
		return ErrInvalidNonce
	}

	return nil
}

func (p *EthProvider) NewTransaction(w *Wallet, params EthTransactionParams) (string, error) {
	if w.Blockchain != p.Blockchain {
		return "", errors.Wrapf(
			ErrUnknownBlockchain,
			"This wallet (%s) doesn't support transactions for %s",
			w.Blockchain,
			p.Blockchain,
		)
	}

	if err := params.validate(); err != nil {
		return "", err
	}

	switch params.Type {
	case Coin:
		return p.newCoinTransaction(w, params)
	case Token:
		return p.newTokenTransaction(w, params)
	default:
		return "", errors.Errorf("unknown transaction type %q", params.Type)
	}
}

// newCoinTransaction creates
func (p *EthProvider) newCoinTransaction(w *Wallet, params EthTransactionParams) (string, error) {
	chainID := big.NewInt(params.NetworkID)

	gasTipCap, err := amountToBigInt(params.MaxPriorityFeePerGas)
	if err != nil {
		return "", errors.Wrap(ErrInvalidGasSettings, "unable to parse MaxPriorityFeePerGas")
	}

	gasFeeCap, err := amountToBigInt(params.MaxFeePerGas)
	if err != nil {
		return "", errors.Wrap(ErrInvalidGasSettings, "unable to parse MaxFeePerGas")
	}

	recipient := common.HexToAddress(params.Recipient)
	amount, err := amountToBigInt(params.Amount)
	if err != nil {
		return "", errors.Wrap(ErrInvalidAmount, "unable to parse Amount")
	}

	privateKey, err := crypto.HexToECDSA(w.PrivateKey[2:])
	if err != nil {
		return "", errors.Wrap(err, "unable to parse private key")
	}

	tx, err := types.SignNewTx(privateKey, types.NewLondonSigner(chainID), &types.DynamicFeeTx{
		ChainID: chainID,
		Nonce:   uint64(params.Nonce),
		To:      &recipient,
		Value:   amount,
		// How to calculate fee: Gas * (GasFeeCap + GasTipCap). Example:
		// 21 000 (18 + 2 gwei) = 21 000 * 20 gwei = 0.00042 ETH ~ 0.69 USD (atm of writing this comment :)
		Gas:       uint64(params.Gas), // e.g. 1 eth tx is ~ 21 000 gas units
		GasFeeCap: gasFeeCap,          // e.g. 20 gwei
		GasTipCap: gasTipCap,          // priorityFee can be 1-2-3 gwei
	})

	if err != nil {
		return "", errors.Wrap(err, "unable to sign ethereum transaction")
	}

	bytes, err := tx.MarshalBinary()
	if err != nil {
		return "", errors.Wrap(err, "unable to encode ethereum transaction")
	}

	return hexutil.Encode(bytes), nil
}

func (p *EthProvider) newTokenTransaction(w *Wallet, params EthTransactionParams) (string, error) {
	chainID := big.NewInt(params.NetworkID)

	gasTipCap, err := amountToBigInt(params.MaxPriorityFeePerGas)
	if err != nil {
		return "", errors.Wrap(ErrInvalidGasSettings, "unable to parse MaxPriorityFeePerGas")
	}

	gasFeeCap, err := amountToBigInt(params.MaxFeePerGas)
	if err != nil {
		return "", errors.Wrap(ErrInvalidGasSettings, "unable to parse MaxFeePerGas")
	}

	contractAddress := common.HexToAddress(params.ContractAddress)
	recipient := common.HexToAddress(params.Recipient)

	amount, err := amountToBigInt(params.Amount)
	if err != nil {
		return "", errors.Wrap(ErrInvalidAmount, "unable to parse Amount")
	}

	privateKey, err := crypto.HexToECDSA(w.PrivateKey[2:])
	if err != nil {
		return "", errors.Wrap(err, "unable to parse private key")
	}

	tx, err := types.SignNewTx(privateKey, types.NewLondonSigner(chainID), &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     uint64(params.Nonce),
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       uint64(params.Gas),
		To:        &contractAddress,
		Value:     big.NewInt(0),
		Data:      constructEthTokenTxData(recipient, amount),
	})

	if err != nil {
		return "", errors.Wrap(err, "unable to sign ethereum transaction")
	}

	bytes, err := tx.MarshalBinary()
	if err != nil {
		return "", errors.Wrap(err, "unable to encode ethereum transaction")
	}

	return hexutil.Encode(bytes), nil
}

func (p *EthProvider) ValidateAddress(address string) bool {
	return validateEthereumAddress(address)
}

func validateEthereumAddress(address string) bool {
	return ethAddressRegex.MatchString(address)
}

func amountToBigInt(raw string) (*big.Int, error) {
	i, set := new(big.Int).SetString(raw, 10)
	if !set {
		return nil, errors.New("unable to make big int")
	}

	return i, nil
}

// see https://goethereumbook.org/en/transfer-tokens/
func constructEthTokenTxData(recipient common.Address, amount *big.Int) []byte {
	hash := sha3.NewLegacyKeccak256()
	hash.Write([]byte(transferFnSignature))
	methodID := hash.Sum(nil)[:4]

	paddedAddress := common.LeftPadBytes(recipient.Bytes(), 32)
	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)

	var data []byte
	data = append(data, methodID...)
	data = append(data, paddedAddress...)
	data = append(data, paddedAmount...)

	return data
}
