package transaction

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgtype"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
)

type Transaction struct {
	ID int64

	CreatedAt time.Time
	UpdatedAt time.Time

	MerchantID int64

	RecipientAddress  string
	RecipientWalletID *int64
	SenderAddress     *string
	SenderWalletID    *int64
	HashID            *string

	Type     Type
	EntityID int64
	Status   Status

	Currency  money.CryptoCurrency
	Amount    money.Money
	USDAmount money.Money

	FactAmount *money.Money

	ServiceFee money.Money
	NetworkFee *money.Money

	IsTest bool

	MetaData MetaData
}

func (tx *Transaction) IsFinalized() bool {
	_, ok := finalizedTransactionStatuses[tx.Status]
	return ok
}

func (tx *Transaction) IsInProgress() bool {
	return tx.Status == StatusInProgress || tx.Status == StatusInProgressInvalid
}

func (tx *Transaction) NetworkID() string {
	return tx.Currency.ChooseNetwork(tx.IsTest)
}

// PaymentLink returns link for payment QR code generation.
func (tx *Transaction) PaymentLink() (string, error) {
	return blockchain.CreatePaymentLink(tx.RecipientAddress, tx.Currency, tx.Amount, tx.IsTest)
}

// ExplorerLink returns link with blockchain explorer e.g. EtherScan
func (tx *Transaction) ExplorerLink() (string, error) {
	if tx.HashID == nil {
		return "", nil
	}

	return blockchain.CreateExplorerTXLink(
		tx.Currency.Blockchain,
		tx.Currency.ChooseNetwork(tx.IsTest),
		*tx.HashID,
	)
}

type MetaData wallet.MetaData

const (
	MetaComment     wallet.MetaDataKey = "comment"
	MetaErrorReason wallet.MetaDataKey = "errorReason"

	MetaTransactionID     = "transactionId"
	MetaRecipientWalletID = "recipientWalletId"
	MetaMerchantID        = "merchantId"
)

func (m MetaData) toJSONB() pgtype.JSONB {
	if len(m) == 0 {
		return pgtype.JSONB{Status: pgtype.Null}
	}

	metaRaw, _ := json.Marshal(m)

	return pgtype.JSONB{Bytes: metaRaw, Status: pgtype.Present}
}

type Status string

const (
	// StatusPending tx was created
	StatusPending Status = "pending"

	// StatusInProgress tx is in progress: blockchain transaction is not confirmed yet
	StatusInProgress Status = "inProgress"

	// StatusInProgressInvalid tx is in progress: blockchain transaction is not confirmed yet
	// but system already sees that amount is not expected
	StatusInProgressInvalid Status = "inProgressInv"

	// StatusCompleted transaction completed
	StatusCompleted Status = "completed"

	// StatusCompletedInvalid transaction completed but not as expected
	// e.g. user deposits more than required
	StatusCompletedInvalid Status = "completedInv"

	// StatusCancelled transaction was canceled w/o actual appearance on blockchain
	StatusCancelled Status = "canceled"

	// StatusFailed transaction was confirmed in blockchain as failed (reverted)
	// but gas was consumed
	StatusFailed Status = "failed"
)

var finalizedTransactionStatuses = map[Status]struct{}{
	StatusCompleted:        {},
	StatusCompletedInvalid: {},
	StatusCancelled:        {},
	StatusFailed:           {},
}

type Type string

const (
	// TypeIncoming is for incoming payments
	TypeIncoming Type = "incoming"

	// TypeInternal is for moving assets from inbound to outbound wallets on blockchain
	TypeInternal Type = "internal"

	// TypeWithdrawal is for moving assets from outbound wallets to merchant's address
	TypeWithdrawal Type = "withdrawal"

	// TypeVirtual is for moving assets within OxygenPay w/o reflecting it on blockchain
	// (e.g. merchant to merchant, system to merchant, ...)
	TypeVirtual Type = "virtual"
)

func (t Type) valid() bool {
	return t == TypeIncoming || t == TypeInternal || t == TypeWithdrawal || t == TypeVirtual
}
