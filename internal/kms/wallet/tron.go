package wallet

import (
	"context"
	"crypto/ecdsa"
	"io"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/provider/trongrid"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/pkg/errors"
)

type TronProvider struct {
	Blockchain   Blockchain
	Trongrid     *trongrid.Provider
	CryptoReader io.Reader
}

type TronTransaction = trongrid.Transaction

type TronTransactionParams struct {
	Type            AssetType
	Recipient       string
	Amount          string
	ContractAddress string
	FeeLimit        uint64
	IsTest          bool
}

var tronAddressRegex = regexp.MustCompile("^T[a-zA-HJ-NP-Z0-9]{33}$")

func (p *TronProvider) Generate() *Wallet {
	key, err := ecdsa.GenerateKey(crypto.S256(), p.CryptoReader)
	if err != nil {
		return &Wallet{}
	}

	// https://developers.tron.network/docs/account#account-address-format
	// This part is the same as ETH address generation.
	privateKey := hexutil.Encode(crypto.FromECDSA(key))
	publicKey := hexutil.Encode(crypto.FromECDSAPub(&key.PublicKey))

	// ETH address has format like 0x123... but TRON uses "41" instead of "0x"
	addressHexString := "41" + crypto.PubkeyToAddress(key.PublicKey).Hex()[2:]
	addressBase58String := util.TronHexToBase58(addressHexString)

	return &Wallet{
		UUID:       uuid.New(),
		CreatedAt:  time.Now(),
		Blockchain: p.Blockchain,
		Address:    addressBase58String,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}
}

func (p *TronProvider) GetBlockchain() Blockchain {
	return p.Blockchain
}

func (p *TronProvider) ValidateAddress(address string) bool {
	return validateTronAddress(address)
}

// Base58ToHexAddress converts from base58 to hex. Example:
// input: TBREsCfBdPyD612xZnwvGPux7osbXvtzLh
// output: 410fe47f49fd91f0edb7fa2b94a3c45d9c2231709c
func (p *TronProvider) Base58ToHexAddress(address string) (string, error) {
	if !p.ValidateAddress(address) {
		return "", ErrInvalidAddress
	}

	bytes := base58.Decode(address)

	return hexutil.Encode(bytes)[2:44], nil
}

func (p TronTransactionParams) validate() error {
	if !p.Type.Valid() {
		return errors.New("type is invalid")
	}

	if !validateTronAddress(p.Recipient) {
		return errors.Wrap(ErrInvalidAddress, "recipient is invalid")
	}

	if p.Type == Token {
		if !validateTronAddress(p.ContractAddress) {
			return ErrInvalidContractAddress
		}

		if p.FeeLimit == 0 {
			return ErrInvalidGasSettings
		}
	}

	if p.Amount == "" {
		return ErrInvalidAmount
	}

	return nil
}

// NewTransaction create new trx / trc20 transaction.
// see https://developers.tron.network/docs/tron-protocol-transaction.
func (p *TronProvider) NewTransaction(
	ctx context.Context,
	wallet *Wallet,
	params TronTransactionParams,
) (TronTransaction, error) {
	if wallet.Blockchain != p.Blockchain {
		return TronTransaction{}, errors.Wrapf(
			ErrUnknownBlockchain,
			"This wallet (%s) doesn't support transactions for %s",
			wallet.Blockchain,
			p.Blockchain,
		)
	}

	if err := params.validate(); err != nil {
		return TronTransaction{}, err
	}

	switch params.Type {
	case Coin:
		return p.newCoinTransaction(ctx, wallet, params)
	case Token:
		return p.newTokenTransaction(ctx, wallet, params)
	default:
		return TronTransaction{}, errors.Errorf("unknown transaction type %q", params.Type)
	}
}

func (p *TronProvider) newCoinTransaction(
	ctx context.Context,
	wallet *Wallet,
	params TronTransactionParams,
) (TronTransaction, error) {
	amountSUN, err := strconv.ParseUint(params.Amount, 10, 64)
	if err != nil {
		return TronTransaction{}, ErrInvalidAmount
	}

	tx, err := p.Trongrid.CreateTransaction(
		ctx,
		trongrid.TransactionRequest{
			OwnerAddress: wallet.Address,
			ToAddress:    params.Recipient,
			Amount:       amountSUN,
			Visible:      true,
		},
		params.IsTest,
	)

	switch {
	case err != nil:
		return TronTransaction{}, err
	case strings.Contains(tx.Error, "balance is not sufficient"):
		return TronTransaction{}, ErrInsufficientBalance
	case tx.Error != "":
		return TronTransaction{}, errors.Wrap(ErrTronResponse, tx.Error)
	}

	if err := p.sign(&tx, wallet); err != nil {
		return TronTransaction{}, errors.Wrap(err, "unable to sign tx")
	}

	return tx, nil
}

func (p *TronProvider) newTokenTransaction(
	ctx context.Context,
	wallet *Wallet,
	params TronTransactionParams,
) (TronTransaction, error) {
	amount, err := amountToBigInt(params.Amount)
	if err != nil {
		return TronTransaction{}, ErrInvalidAmount
	}

	callParameterHex, err := p.constructTronTokenTxData(params.Recipient, amount)
	if err != nil {
		return TronTransaction{}, errors.Wrap(err, "unable to construct contract payload")
	}

	tx, err := p.Trongrid.CallContract(
		ctx,
		trongrid.ContractCallRequest{
			OwnerAddress:     wallet.Address,
			ContractAddress:  params.ContractAddress,
			FunctionSelector: transferFnSignature,
			Parameter:        callParameterHex,
			FeeLimit:         params.FeeLimit,
			CallValue:        0,
			Visible:          true,
		},
		params.IsTest,
	)

	switch {
	case errors.Is(err, trongrid.ErrResponse):
		if strings.Contains(err.Error(), "balance is not sufficient") {
			return TronTransaction{}, ErrInsufficientBalance
		}
	case err != nil:
		return TronTransaction{}, err
	}

	if err := p.sign(&tx, wallet); err != nil {
		return TronTransaction{}, errors.Wrap(err, "unable to sign tx")
	}

	return tx, nil
}

func (p *TronProvider) sign(tx *trongrid.Transaction, wallet *Wallet) error {
	privateKey, err := crypto.HexToECDSA(wallet.PrivateKey[2:])
	if err != nil {
		return errors.Wrap(err, "unable to decode private key")
	}

	bytes, err := hexutil.Decode("0x" + tx.RawDataHex)
	if err != nil {
		return errors.Wrap(err, "unable to decode rawDataHex")
	}

	hash := util.SHA256(bytes)

	signature, err := crypto.Sign(hash, privateKey)
	if err != nil {
		return errors.Wrap(err, "unable to sign tx")
	}

	tx.Signature = append(tx.Signature, hexutil.Encode(signature)[2:])

	return nil
}

// constructTronTokenTxData the same as constructEthTokenTxData but w/o methodID hex prefix
func (p *TronProvider) constructTronTokenTxData(recipientBase58 string, amount *big.Int) (string, error) {
	recipientHexString, err := p.Base58ToHexAddress(recipientBase58)
	if err != nil {
		return "", err
	}

	recipient := common.HexToAddress(recipientHexString)

	paddedAddress := common.LeftPadBytes(recipient.Bytes(), 32)
	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)

	var data []byte
	data = append(data, paddedAddress...)
	data = append(data, paddedAmount...)

	return hexutil.Encode(data)[2:], nil
}

func validateTronAddress(address string) bool {
	return tronAddressRegex.MatchString(address)
}
