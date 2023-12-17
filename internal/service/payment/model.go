package payment

import (
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/util"
)

type Payment struct {
	ID       int64
	PublicID uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time
	ExpiresAt *time.Time

	Type   Type
	Status Status

	MerchantID        int64
	MerchantOrderUUID uuid.UUID
	MerchantOrderID   *string

	Price money.Money

	RedirectURL   string
	PaymentURL    string
	WebhookSentAt *time.Time

	Description *string
	IsTest      bool

	CustomerID *int64

	metadata Metadata
}

type Metadata = wallet.MetaData

const (
	MetaBalanceID wallet.MetaDataKey = "balanceID"
	MetaAddressID wallet.MetaDataKey = "addressID"

	MetaInternalPayment wallet.MetaDataKey = "internalPayment"

	MetaLinkID             wallet.MetaDataKey = "linkID"
	MetaLinkSuccessAction  wallet.MetaDataKey = "linkSuccessAction"
	MetaLinkSuccessMessage wallet.MetaDataKey = "linkSuccessMessage"
)

// IsEditable checks that payment can be edited
// (e.g. customer assignment/ selecting payment method)
func (p *Payment) IsEditable() bool {
	return p.Status == StatusPending
}

// PublicStatus returns payment's status for public output in the API.
func (p *Payment) PublicStatus() Status {
	switch p.Status {
	case StatusPending, StatusLocked:
		return StatusPending
	default:
		return p.Status
	}
}

func (p *Payment) receivedPayment() bool {
	switch p.PublicStatus() {
	case StatusInProgress, StatusSuccess:
		return true
	default:
		return false
	}
}

// PublicSuccessURL returns successURL or nil based on status.
func (p *Payment) PublicSuccessURL() *string {
	if !p.receivedPayment() {
		return nil
	}

	successAction := p.PublicSuccessAction()
	if successAction != nil && *successAction != SuccessActionRedirect {
		return nil
	}

	return &p.RedirectURL
}

func (p *Payment) PublicSuccessAction() *SuccessAction {
	if !p.receivedPayment() {
		return nil
	}

	if p.LinkSuccessAction() != nil {
		return p.LinkSuccessAction()
	}

	// default case
	return util.Ptr(SuccessActionRedirect)
}

func (p *Payment) PublicSuccessMessage() *string {
	if !p.receivedPayment() {
		return nil
	}

	return p.LinkSuccessMessage()
}

func (p *Payment) WithdrawalBalanceID() int64 {
	i, _ := strconv.Atoi(p.metadata[MetaBalanceID])
	return int64(i)
}

func (p *Payment) WithdrawalAddressID() int64 {
	i, _ := strconv.Atoi(p.metadata[MetaAddressID])
	return int64(i)
}

func (p *Payment) ExpirationDurationMin() int64 {
	return int64(ExpirationPeriodForLocked / time.Minute)
}

func (p *Payment) LinkID() int64 {
	i, _ := strconv.Atoi(p.metadata[MetaLinkID])
	return int64(i)
}

func (p *Payment) LinkSuccessAction() *SuccessAction {
	if s, ok := p.metadata[MetaLinkSuccessAction]; ok {
		action := SuccessAction(s)
		return &action
	}

	return nil
}

func (p *Payment) LinkSuccessMessage() *string {
	if s, ok := p.metadata[MetaLinkSuccessMessage]; ok {
		return &s
	}

	return nil
}

type Type string

const (
	TypePayment    Type = "payment"
	TypeWithdrawal Type = "withdrawal"
)

func (t Type) String() string {
	return string(t)
}

type Status string

const (
	// StatusPending just created
	StatusPending Status = "pending"

	// StatusLocked customer filled the form
	StatusLocked Status = "locked"

	// StatusInProgress underlying tx is in progress
	StatusInProgress Status = "inProgress"

	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
)

func (s Status) String() string {
	return string(s)
}

// Method payment method.
type Method struct {
	Currency      money.CryptoCurrency
	TransactionID int64
	NetworkID     string
	IsTest        bool

	tx *transaction.Transaction
}

func (m *Method) TX() *transaction.Transaction {
	return m.tx
}

type Customer struct {
	ID         int64
	MerchantID int64
	UUID       uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time

	Email string
}

type CustomerDetails struct {
	Customer
	SuccessfulPayments int64
	RecentPayments     []*Payment
}
