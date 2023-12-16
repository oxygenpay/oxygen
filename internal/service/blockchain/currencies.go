package blockchain

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"

	kms "github.com/oxygenpay/oxygen/internal/kms/wallet"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/pkg/errors"
)

type Resolver interface {
	ListSupportedCurrencies(withDeprecated bool) []money.CryptoCurrency
	ListBlockchainCurrencies(blockchain money.Blockchain) []money.CryptoCurrency
	GetCurrencyByTicker(ticker string) (money.CryptoCurrency, error)
	GetNativeCoin(blockchain money.Blockchain) (money.CryptoCurrency, error)
	GetCurrencyByBlockchainAndContract(bc money.Blockchain, networkID, addr string) (money.CryptoCurrency, error)
	GetMinimalWithdrawalByTicker(ticker string) (money.Money, error)
	GetUSDMinimalInternalTransferByTicker(ticker string) (money.Money, error)
}

type CurrencyResolver struct {
	mu                       sync.RWMutex
	currencies               map[string]money.CryptoCurrency
	minimalWithdrawals       map[string]money.Money
	minimalInternalTransfers map[string]money.Money
	currencyBlockchains      map[string]map[money.Blockchain]struct{}
	blockchainCurrencies     map[money.Blockchain]map[string]struct{}
}

func NewCurrencies() *CurrencyResolver {
	return &CurrencyResolver{
		mu:                       sync.RWMutex{},
		currencies:               make(map[string]money.CryptoCurrency),
		minimalWithdrawals:       make(map[string]money.Money),
		minimalInternalTransfers: make(map[string]money.Money),
		currencyBlockchains:      make(map[string]map[money.Blockchain]struct{}),
		blockchainCurrencies:     make(map[money.Blockchain]map[string]struct{}),
	}
}

func (r *CurrencyResolver) GetCurrencyByTicker(ticker string) (money.CryptoCurrency, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ticker = strings.ToUpper(ticker)

	c, ok := r.currencies[ticker]
	if !ok {
		return money.CryptoCurrency{}, errors.Wrap(ErrCurrencyNotFound, ticker)
	}

	return c, nil
}

// GetNativeCoin returns native coin by blockchain. Example: ETH -> ETH; BSC -> BNB.
func (r *CurrencyResolver) GetNativeCoin(chain money.Blockchain) (money.CryptoCurrency, error) {
	list := r.ListBlockchainCurrencies(chain)

	for i := range list {
		if list[i].Type == money.Coin {
			return list[i], nil
		}
	}

	return money.CryptoCurrency{}, ErrCurrencyNotFound
}

// GetMinimalWithdrawalByTicker returns minimal withdrawal amount in USD for selected ticker.
func (r *CurrencyResolver) GetMinimalWithdrawalByTicker(ticker string) (money.Money, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	amount, ok := r.minimalWithdrawals[ticker]
	if !ok {
		return money.Money{}, ErrCurrencyNotFound
	}

	return amount, nil
}

// GetUSDMinimalInternalTransferByTicker returns minimal internal transfer amount in USD for selected ticker.
// internal transfer is a transfer from inbound to outbound O2pay wallet.
func (r *CurrencyResolver) GetUSDMinimalInternalTransferByTicker(ticker string) (money.Money, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	amount, ok := r.minimalInternalTransfers[ticker]
	if !ok {
		return money.Money{}, ErrCurrencyNotFound
	}

	return amount, nil
}

// GetCurrencyByBlockchainAndContract searches currency by blockchain
// and contract address across both mainnet & testnet.
//
//nolint:gocritic
func (r *CurrencyResolver) GetCurrencyByBlockchainAndContract(bc money.Blockchain, networkID, addr string) (money.CryptoCurrency, error) {
	if bc == "" || networkID == "" || addr == "" {
		return money.CryptoCurrency{}, errors.Wrap(ErrCurrencyNotFound, "invalid input")
	}

	addr = strings.ToLower(addr)

	for _, c := range r.ListBlockchainCurrencies(bc) {
		if c.Type != money.Token {
			continue
		}

		if c.NetworkID == networkID {
			// mainnet
			if strings.ToLower(c.TokenContractAddress) == addr {
				return c, nil
			}
		} else {
			// testnet
			if strings.ToLower(c.TestTokenContractAddress) == addr {
				return c, nil
			}
		}

		for _, a := range c.Aliases {
			if a == addr {
				return c, nil
			}
		}
	}

	return money.CryptoCurrency{}, ErrCurrencyNotFound
}

func (r *CurrencyResolver) ListSupportedCurrencies(withDeprecated bool) []money.CryptoCurrency {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]money.CryptoCurrency, 0)
	for i := range r.currencies {
		if r.currencies[i].Deprecated && !withDeprecated {
			continue
		}

		results = append(results, r.currencies[i])
	}

	return sortCurrencies(results)
}

func (r *CurrencyResolver) ListSupportedBlockchains() []money.Blockchain {
	r.mu.RLock()
	defer r.mu.RUnlock()

	blockchains := util.Keys(r.blockchainCurrencies)

	sort.Slice(blockchains, func(i, j int) bool { return blockchains[i] < blockchains[j] })

	return blockchains
}

func (r *CurrencyResolver) ListBlockchainCurrencies(blockchain money.Blockchain) []money.CryptoCurrency {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]money.CryptoCurrency, 0)
	for ticker := range r.blockchainCurrencies[blockchain] {
		results = append(results, r.currencies[ticker])
	}

	return sortCurrencies(results)
}

func (r *CurrencyResolver) addCurrency(currency money.CryptoCurrency) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.currencies[currency.Ticker] = currency

	r.ensureIndices(currency.Blockchain, currency.Ticker)

	if currency.Deprecated {
		return
	}

	// add currency to "index"
	r.blockchainCurrencies[currency.Blockchain][currency.Ticker] = struct{}{}

	// add currency to "reverse index"
	r.currencyBlockchains[currency.Ticker][currency.Blockchain] = struct{}{}
}

func (r *CurrencyResolver) addMinimalWithdrawal(ticker string, amount money.Money) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.minimalWithdrawals[ticker] = amount
}

func (r *CurrencyResolver) addMinimalInternalTransfer(ticker string, amount money.Money) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.minimalInternalTransfers[ticker] = amount
}

func (r *CurrencyResolver) ensureIndices(blockchain money.Blockchain, ticker string) {
	if r.blockchainCurrencies[blockchain] == nil {
		r.blockchainCurrencies[blockchain] = make(map[string]struct{})
	}

	if r.currencyBlockchains[ticker] == nil {
		r.currencyBlockchains[ticker] = make(map[money.Blockchain]struct{})
	}
}

func sortCurrencies(currencies []money.CryptoCurrency) []money.CryptoCurrency {
	sort.Slice(currencies, func(i, j int) bool {
		if currencies[i].Name == currencies[j].Name {
			return currencies[i].Ticker < currencies[j].Ticker
		}

		return currencies[i].Name < currencies[j].Name
	})

	return currencies
}

//go:embed currencies.json
var currencies embed.FS

// nolint:gocritic
// DefaultSetup applied coins & tokens that are supported within Oxygen.
func DefaultSetup(s *CurrencyResolver) error {
	file, err := currencies.Open("currencies.json")
	if err != nil {
		return err
	}

	defer file.Close()

	raw, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	var currencies []map[string]string

	if err := json.Unmarshal(raw, &currencies); err != nil {
		return err
	}

	for _, c := range currencies {
		decimals, err := strconv.Atoi(c["decimals"])
		if err != nil {
			return err
		}

		moneyType := money.Coin
		if c["type"] == string(money.Token) {
			moneyType = money.Token
		}

		tokenAddr := c["tokenAddress"]
		if moneyType == money.Token && tokenAddr == "" {
			return errors.Wrap(ErrNoTokenAddress, c["ticker"])
		}

		// test token might be absent
		testTokenAddr := c["testTokenAddress"]

		// Some token currencies from Tatum webhooks come with their ticker instead of contract address
		// That's their mistake, but we still need to deal with
		aliases := parseAliases(c["aliases"])

		minimalWithdrawal, err := parseUSD(c["minimal_withdrawal_amount_usd"])
		if err != nil {
			return err
		}

		minimalInternalTransfer, err := parseUSD(c["minimal_instant_internal_transfer_amount_usd"])
		if err != nil {
			return err
		}

		deprecated, err := parseBool(c["deprecated"])
		if err != nil {
			return err
		}

		ticker := c["ticker"]

		s.addCurrency(money.CryptoCurrency{
			Blockchain:               money.Blockchain(c["blockchain"]),
			BlockchainName:           c["blockchainName"],
			NetworkID:                c["networkId"],
			TestNetworkID:            c["testNetworkId"],
			Type:                     moneyType,
			Ticker:                   ticker,
			Name:                     c["name"],
			TokenContractAddress:     tokenAddr,
			TestTokenContractAddress: testTokenAddr,
			Aliases:                  aliases,
			Decimals:                 int64(decimals),
			Deprecated:               deprecated,
		})

		s.addMinimalWithdrawal(ticker, minimalWithdrawal)
		s.addMinimalInternalTransfer(ticker, minimalInternalTransfer)
	}

	return nil
}

func CreatePaymentLink(addr string, currency money.CryptoCurrency, amount money.Money, isTest bool) (string, error) {
	switch kms.Blockchain(currency.Blockchain) {
	case kms.ETH, kms.MATIC, kms.BSC:
		return ethPaymentLink(addr, currency, amount, isTest), nil
	case kms.TRON:
		return tronPaymentLink(addr, currency, amount, isTest), nil
	}

	return "", errors.Errorf("unable to create payment link for %s", currency.Blockchain)
}

// https://github.com/ethereum/EIPs/blob/master/EIPS/eip-681.md
func ethPaymentLink(addr string, currency money.CryptoCurrency, amount money.Money, isTest bool) string {
	var link string
	if currency.Type == money.Coin {
		link = fmt.Sprintf("ethereum:%s@%s?value=%s",
			addr,
			currency.ChooseNetwork(isTest),
			amount.StringRaw(),
		)
	} else {
		link = fmt.Sprintf("ethereum:%s@%s/transfer?address=%s&uint256=%s",
			currency.ChooseContractAddress(isTest),
			currency.ChooseNetwork(isTest),
			addr,
			amount.StringRaw(),
		)
	}

	return link
}

// Tron has no standards in QR-codes, so in this data we can't really reflect TRC20 case when dealing with tokens.
func tronPaymentLink(addr string, _ money.CryptoCurrency, amount money.Money, _ bool) string {
	return fmt.Sprintf("tron:%s?amount=%s", addr, amount.String())
}

var explorers = map[string]string{
	"ETH/1":        "https://etherscan.io/tx/%s",
	"ETH/5":        "https://goerli.etherscan.io/tx/%s",
	"MATIC/137":    "https://polygonscan.com/tx/%s",
	"MATIC/80001":  "https://mumbai.polygonscan.com/tx/%s",
	"BSC/56":       "https://bscscan.com/tx/%s",
	"BSC/97":       "https://testnet.bscscan.com/tx/%s",
	"TRON/mainnet": "https://tronscan.org/#/transaction/%s",
	"TRON/testnet": "https://shasta.tronscan.org/#/transaction/%s",
}

func CreateExplorerTXLink(blockchain money.Blockchain, networkID, txID string) (string, error) {
	key := fmt.Sprintf("%s/%s", blockchain.String(), networkID)

	tpl, ok := explorers[key]
	if !ok {
		return "", ErrCurrencyNotFound
	}

	return fmt.Sprintf(tpl, txID), nil
}

func parseUSD(raw string) (money.Money, error) {
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return money.Money{}, errors.Wrap(ErrParseMoney, raw)
	}

	return money.FiatFromFloat64(money.USD, f)
}

func parseBool(raw string) (bool, error) {
	if raw == "" {
		return false, nil
	}

	return strconv.ParseBool(raw)
}

func parseAliases(raw string) []string {
	if raw == "" {
		return nil
	}

	aliases := strings.Split(strings.ToLower(raw), ",")

	return util.MapSlice(aliases, strings.TrimSpace)
}
