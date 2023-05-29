package wallet

import (
	"io"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/wemeetagain/go-hdwallet"
)

type BitcoinProvider struct {
	Blockchain   Blockchain
	CryptoReader io.Reader
}

var bitcoinAddressRegex = regexp.MustCompile("^(bc1|[13])[a-zA-HJ-NP-Z0-9]{25,39}$")

func (p *BitcoinProvider) Generate() *Wallet {
	seed := make([]byte, 256)
	if _, err := io.ReadFull(p.CryptoReader, seed); err != nil {
		return &Wallet{}
	}

	privateKey := hdwallet.MasterKey(seed)
	publicKey := privateKey.Pub()
	address := publicKey.Address()

	return &Wallet{
		UUID:       uuid.New(),
		CreatedAt:  time.Now(),
		Blockchain: p.Blockchain,
		Address:    address,
		PublicKey:  publicKey.String(),
		PrivateKey: privateKey.String(),
	}
}

func (p *BitcoinProvider) GetBlockchain() Blockchain {
	return p.Blockchain
}

func (p *BitcoinProvider) ValidateAddress(address string) bool {
	return validateBitcoinAddress(address)
}

func validateBitcoinAddress(address string) bool {
	return bitcoinAddressRegex.MatchString(address)
}
