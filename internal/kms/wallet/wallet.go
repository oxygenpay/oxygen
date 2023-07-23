// Package wallet is used inside KMS to provide features related to wallet generation & CRUD access.
package wallet

import (
	"time"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/pkg/errors"
)

type Blockchain string

const (
	BTC   Blockchain = "BTC"
	ETH   Blockchain = "ETH"
	TRON  Blockchain = "TRON"
	MATIC Blockchain = "MATIC"
	BSC   Blockchain = "BSC"
)

var blockchains = []Blockchain{BTC, ETH, TRON, MATIC, BSC}

func ListBlockchains() []Blockchain {
	result := make([]Blockchain, len(blockchains))
	copy(result, blockchains)

	return result
}

type Wallet struct {
	UUID       uuid.UUID  `json:"uuid"`
	Address    string     `json:"address"`
	PublicKey  string     `json:"public_key"`
	PrivateKey string     `json:"private_key"`
	CreatedAt  time.Time  `json:"created_at"`
	DeletedAt  *time.Time `json:"deleted_at"`
	Blockchain Blockchain `json:"blockchain"`
}

func (b Blockchain) IsValid() bool {
	for _, bc := range blockchains {
		if b == bc {
			return true
		}
	}

	return false
}

func (b Blockchain) ToMoneyBlockchain() money.Blockchain {
	return money.Blockchain(b)
}

func (b Blockchain) String() string {
	return string(b)
}

func (b Blockchain) NotSpecified() bool {
	return b == ""
}

func (b Blockchain) IsSpecified() bool {
	return b != ""
}

func ValidateAddress(blockchain Blockchain, address string) error {
	var isValid bool
	switch blockchain {
	case BTC:
		isValid = validateBitcoinAddress(address)
	case ETH, MATIC, BSC:
		isValid = validateEthereumAddress(address)
	case TRON:
		isValid = validateTronAddress(address)
	default:
		return errors.Wrapf(ErrUnknownBlockchain, "unknown blockchain %q", blockchain)
	}

	if !isValid {
		return ErrInvalidAddress
	}

	return nil
}
