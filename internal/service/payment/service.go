package payment

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"net/mail"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/bus"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/service/blockchain"
	"github.com/oxygenpay/oxygen/internal/service/merchant"
	"github.com/oxygenpay/oxygen/internal/service/transaction"
	"github.com/oxygenpay/oxygen/internal/service/wallet"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

type BlockchainService interface {
	blockchain.Resolver
	blockchain.Convertor
	blockchain.FeeCalculator
}

type TransactionResolver interface {
	GetLatestByPaymentID(ctx context.Context, paymentID int64) (*transaction.Transaction, error)
	EagerLoadByPaymentIDs(ctx context.Context, merchantID int64, paymentIDs []int64) ([]*transaction.Transaction, error)
}

type Service struct {
	repo         *repository.Queries
	basePath     string
	logger       *zerolog.Logger
	transactions TransactionResolver
	merchants    *merchant.Service
	wallets      *wallet.Service
	blockchain   BlockchainService
	publisher    bus.Publisher
}

// ExpirationPeriodForLocked expiration period for incoming payment when locked
const ExpirationPeriodForLocked = time.Minute * 20

// ExpirationPeriodForNotLocked expiration period for non-locked payment
// e.g. when payment is created but user haven't opened the page or haven't locked a cryptocurrency.
const ExpirationPeriodForNotLocked = time.Hour * 6

const MerchantIDWildcard = transaction.MerchantIDWildcard

const (
	limitDefault = 50
	limitMax     = 100
)

var (
	ErrNotFound                      = errors.New("not found")
	ErrAlreadyExists                 = errors.New("payment already exists")
	ErrValidation                    = errors.New("payment is invalid")
	ErrLinkValidation                = errors.New("payment link is invalid")
	ErrPaymentMethodNotSet           = errors.New("payment method is not set yet")
	ErrPaymentLocked                 = errors.New("payment is locked for editing")
	ErrInvalidLimit                  = errors.New("invalid limit")
	ErrAddressBalanceMismatch        = errors.New("selected address does not match with balance")
	ErrWithdrawalInsufficientBalance = errors.New("not enough funds")
	ErrWithdrawalAmountTooSmall      = errors.New("withdrawal amount is too small")
)

func New(
	repo *repository.Queries,
	basePath string,
	transactionService TransactionResolver,
	merchantService *merchant.Service,
	walletService *wallet.Service,
	blockchainService BlockchainService,
	publisher bus.Publisher,
	logger *zerolog.Logger,
) *Service {
	log := logger.With().Str("channel", "payment_service").Logger()

	return &Service{
		repo:         repo,
		basePath:     basePath,
		transactions: transactionService,
		merchants:    merchantService,
		wallets:      walletService,
		blockchain:   blockchainService,
		publisher:    publisher,
		logger:       &log,
	}
}

func (s *Service) GetByID(ctx context.Context, merchantID, id int64) (*Payment, error) {
	// merchantID here is for validating that this payment really belongs to the merchant.
	p, err := s.repo.GetPaymentByID(ctx, repository.GetPaymentByIDParams{
		ID:                 id,
		MerchantID:         merchantID,
		FilterByMerchantID: merchantID != MerchantIDWildcard,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return s.entryToPayment(p)
}

func (s *Service) GetByMerchantOrderID(
	ctx context.Context,
	merchantID int64,
	merchantOrderUUID uuid.UUID,
) (*Payment, error) {
	p, err := s.repo.GetPaymentByMerchantIDAndOrderUUID(ctx, repository.GetPaymentByMerchantIDAndOrderUUIDParams{
		MerchantID:        merchantID,
		MerchantOrderUuid: merchantOrderUUID,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return s.entryToPayment(p)
}

func (s *Service) GetByMerchantOrderIDWithRelations(
	ctx context.Context,
	merchantID int64,
	merchantOrderUUID uuid.UUID,
) (PaymentWithRelations, error) {
	pt, err := s.GetByMerchantOrderID(ctx, merchantID, merchantOrderUUID)
	if err != nil {
		return PaymentWithRelations{}, err
	}

	slice := []PaymentWithRelations{{Payment: pt}}

	if err := s.eagerLoadRelations(ctx, merchantID, slice); err != nil {
		return PaymentWithRelations{}, err
	}

	return slice[0], nil
}

func (s *Service) GetByPublicID(ctx context.Context, publicID uuid.UUID) (*Payment, error) {
	p, err := s.repo.GetPaymentByPublicID(ctx, publicID)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return s.entryToPayment(p)
}

func (s *Service) GetByMerchantIDs(ctx context.Context, merchantID int64, merchantOrderUUID uuid.UUID) (*Payment, error) {
	p, err := s.repo.GetPaymentByMerchantIDs(ctx, repository.GetPaymentByMerchantIDsParams{
		MerchantID:        merchantID,
		MerchantOrderUuid: merchantOrderUUID,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return s.entryToPayment(p)
}

type ListOptions struct {
	Limit        int
	Cursor       string
	ReverseOrder bool
	FilterByType []Type
}

// List paginates payments by provided merchantID and ListOptions.
func (s *Service) List(ctx context.Context, merchantID int64, opts ListOptions) ([]*Payment, string, error) {
	// 1. setup limit
	limit := int32(opts.Limit)
	if limit == 0 {
		limit = limitDefault
	}

	if limit > limitMax {
		return nil, "", ErrInvalidLimit
	}

	// 2. resolve cursor. If cursor provided, then we need to map cursor to payment.id
	var cursorPayment *Payment
	if opts.Cursor != "" {
		paymentID, err := uuid.Parse(opts.Cursor)
		if err != nil {
			return nil, "", errors.Wrap(ErrValidation, "invalid cursor")
		}

		p, err := s.GetByMerchantIDs(ctx, merchantID, paymentID)
		if err != nil {
			return nil, "", errors.Wrap(err, "unable to get fromID payment")
		}

		cursorPayment = p
	}

	// 3. map filter
	filterByType := util.MapSlice(opts.FilterByType, func(t Type) string { return string(t) })

	var results []repository.Payment
	var err error

	if opts.ReverseOrder {
		fromID := int64(math.MaxInt64)
		if cursorPayment != nil {
			fromID = cursorPayment.ID
		}

		results, err = s.repo.PaginatePaymentsDesc(ctx, repository.PaginatePaymentsDescParams{
			MerchantID:    merchantID,
			ID:            fromID,
			Limit:         limit + 1,
			FilterByTypes: len(filterByType) > 0,
			Type:          filterByType,
		})
	} else {
		var fromID int64
		if cursorPayment != nil {
			fromID = cursorPayment.ID
		}

		results, err = s.repo.PaginatePaymentsAsc(ctx, repository.PaginatePaymentsAscParams{
			MerchantID:    merchantID,
			ID:            fromID,
			Limit:         limit + 1,
			FilterByTypes: len(filterByType) > 0,
			Type:          filterByType,
		})
	}

	if err != nil {
		return nil, "", errors.Wrap(err, "unable to paginate payments")
	}

	// 3. map results
	payments, err := s.entriesToPayments(results)
	if err != nil {
		return nil, "", errors.Wrap(err, "unable to map payments")
	}

	// 4. in case of 'limit + 1' entries last item is
	// next cursor => resolve nextCursor and remove it from results.
	var nextCursor string
	if len(payments) > int(limit) {
		nextCursor = payments[len(payments)-1].MerchantOrderUUID.String()
		payments = payments[:limit]
	}

	return payments, nextCursor, nil
}

//nolint:revive
type PaymentWithRelations struct {
	Payment     *Payment
	Transaction *transaction.Transaction
	Customer    *Customer
	Address     *merchant.Address
	Balance     *wallet.Balance
}

// ListWithRelations paginates payments with loaded relations.
func (s *Service) ListWithRelations(ctx context.Context, merchantID int64, opt ListOptions) ([]PaymentWithRelations, string, error) {
	payments, cursor, err := s.List(ctx, merchantID, opt)
	if err != nil {
		return nil, "", errors.Wrap(err, "unable to list payments")
	}

	pagination := util.MapSlice(payments, func(pt *Payment) PaymentWithRelations {
		return PaymentWithRelations{Payment: pt}
	})

	if err := s.eagerLoadRelations(ctx, merchantID, pagination); err != nil {
		return nil, "", err
	}

	return pagination, cursor, nil
}

func (s *Service) eagerLoadRelations(ctx context.Context, merchantID int64, payments []PaymentWithRelations) error {
	if err := s.eagerLoadTransactions(ctx, merchantID, payments); err != nil {
		return errors.Wrap(err, "unable to eager load transactions")
	}

	if err := s.eagerLoadCustomers(ctx, merchantID, payments); err != nil {
		return errors.Wrap(err, "unable to eager load customers")
	}

	if err := s.eagerLoadAddresses(ctx, merchantID, payments); err != nil {
		return errors.Wrap(err, "unable to eager load addresses")
	}

	if err := s.eagerLoadBalances(ctx, merchantID, payments); err != nil {
		return errors.Wrap(err, "unable to eager load balances")
	}

	return nil
}

func (s *Service) eagerLoadTransactions(ctx context.Context, merchantID int64, pagination []PaymentWithRelations) error {
	paymentIDs := util.MapSlice(pagination, func(p PaymentWithRelations) int64 { return p.Payment.ID })

	txs, err := s.transactions.EagerLoadByPaymentIDs(ctx, merchantID, paymentIDs)
	if err != nil {
		return err
	}

	// 1. map txs by [paymentID: tx]
	txMap := util.KeyFunc(txs, func(tx *transaction.Transaction) int64 { return tx.EntityID })
	if len(txMap) == 0 {
		return nil
	}

	for i := range pagination {
		tx, ok := txMap[pagination[i].Payment.ID]
		if ok {
			pagination[i].Transaction = tx
		}
	}

	return nil
}

func (s *Service) eagerLoadCustomers(ctx context.Context, merchantID int64, pagination []PaymentWithRelations) error {
	customerIDs := util.MapSlice(pagination, func(p PaymentWithRelations) int64 {
		if p.Payment.CustomerID == nil {
			return 0
		}

		return *p.Payment.CustomerID
	})

	customers, err := s.GetBatchCustomers(ctx, merchantID, customerIDs)
	if err != nil {
		return err
	}

	customersByID := util.KeyFunc(customers, func(c *Customer) int64 { return c.ID })
	if len(customerIDs) == 0 {
		return nil
	}

	for i := range pagination {
		if pagination[i].Payment.CustomerID == nil {
			continue
		}

		c, ok := customersByID[*pagination[i].Payment.CustomerID]
		if ok {
			pagination[i].Customer = c
		}
	}

	return nil
}

func (s *Service) eagerLoadAddresses(ctx context.Context, merchantID int64, payments []PaymentWithRelations) error {
	addresses, err := s.merchants.ListMerchantAddresses(ctx, merchantID)
	if err != nil {
		return err
	}

	addressesByID := util.KeyFunc(addresses, func(c *merchant.Address) int64 { return c.ID })
	if len(addressesByID) == 0 {
		return nil
	}

	for i := range payments {
		addr, ok := addressesByID[payments[i].Payment.WithdrawalAddressID()]
		if ok {
			payments[i].Address = addr
		}
	}

	return nil
}

func (s *Service) eagerLoadBalances(ctx context.Context, merchantID int64, payments []PaymentWithRelations) error {
	balances, err := s.wallets.ListBalances(ctx, wallet.EntityTypeMerchant, merchantID, false)
	if err != nil {
		return err
	}

	balancesByID := util.KeyFunc(balances, func(c *wallet.Balance) int64 { return c.ID })

	for i := range payments {
		balance, ok := balancesByID[payments[i].Payment.WithdrawalBalanceID()]
		if ok {
			payments[i].Balance = balance
		}
	}

	return nil
}

// GetBatchExpired returns list of expired payments. An expired payment is a payment that either has
// (expires_at != null && expires_at < $ExpiresAt) || (expires_at is null && created_at < $CreatedAt)
func (s *Service) GetBatchExpired(ctx context.Context, limit int64) ([]*Payment, error) {
	lim := int32(limit)
	if lim == 0 {
		lim = limitDefault
	}

	results, err := s.repo.GetBatchExpiredPayments(ctx, repository.GetBatchExpiredPaymentsParams{
		ExpiresAt: repository.TimeToNullable(time.Now()),
		CreatedAt: time.Now().Add(-ExpirationPeriodForNotLocked),
		Type:      string(TypePayment),
		Status:    []string{StatusPending.String(), StatusLocked.String()},
		Limit:     lim,
	})
	if err != nil {
		return nil, err
	}

	payments := make([]*Payment, len(results))
	for i := range results {
		p, err := s.entryToPayment(results[i])
		if err != nil {
			return nil, err
		}

		payments[i] = p
	}

	return payments, nil
}

type CreateOpt func(props *CreatePaymentProps)

func FromLink(link *Link) CreateOpt {
	return func(p *CreatePaymentProps) {
		p.fromLink = true
		p.linkID = link.ID
		p.linkSuccessAction = link.SuccessAction
		p.linkSuccessMessage = link.SuccessMessage
	}
}

func (s *Service) CreatePayment(
	ctx context.Context,
	merchantID int64,
	props CreatePaymentProps,
	opts ...CreateOpt,
) (*Payment, error) {
	for _, opt := range opts {
		opt(&props)
	}

	if err := props.validate(); err != nil {
		return nil, err
	}

	mt, err := s.merchants.GetByID(ctx, merchantID, false)
	if err != nil {
		return nil, err
	}

	if _, errGet := s.GetByMerchantOrderID(ctx, merchantID, props.MerchantOrderUUID); errGet == nil {
		return nil, ErrAlreadyExists
	}

	price, decimals := props.Money.BigInt()

	now := time.Now()

	redirectURL := mt.Website
	if props.RedirectURL != nil {
		redirectURL = *props.RedirectURL
	}

	meta := make(Metadata)

	if props.fromLink {
		meta = fillPaymentMetaWithLink(meta, props)
	}

	p, err := s.repo.CreatePayment(ctx, repository.CreatePaymentParams{
		PublicID: uuid.New(),

		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: sql.NullTime{},

		Type:   TypePayment.String(),
		Status: StatusPending.String(),

		MerchantID:        merchantID,
		MerchantOrderUuid: props.MerchantOrderUUID,
		MerchantOrderID:   repository.PointerStringToNullable(props.MerchantOrderID),

		Price:    repository.BigIntToNumeric(price),
		Decimals: int32(decimals),
		Currency: props.Money.Ticker(),

		RedirectUrl: redirectURL,

		Description: repository.PointerStringToNullable(props.Description),
		IsTest:      props.IsTest,
		Metadata:    meta.ToJSONB(),
	})

	if err != nil {
		return nil, err
	}

	return s.entryToPayment(p)
}

type CreateInternalPaymentProps struct {
	MerchantOrderUUID uuid.UUID
	Money             money.Money
	Description       string
	IsTest            bool
}

// CreateSystemTopup creates an internal payment that is reflected only within OxygenPay.
// This payment is treated as successful.
func (s *Service) CreateSystemTopup(ctx context.Context, merchantID int64, props CreateInternalPaymentProps) (*Payment, error) {
	if props.Money.Type() != money.Crypto {
		return nil, errors.Wrap(ErrValidation, "internal payments should be in crypto")
	}

	mt, err := s.merchants.GetByID(ctx, merchantID, false)
	if err != nil {
		return nil, err
	}

	if _, errGet := s.GetByMerchantOrderID(ctx, merchantID, props.MerchantOrderUUID); errGet == nil {
		return nil, ErrAlreadyExists
	}

	var (
		price, decimals = props.Money.BigInt()
		now             = time.Now()
		meta            = Metadata{MetaInternalPayment: "system topup"}
	)

	pt, err := s.repo.CreatePayment(ctx, repository.CreatePaymentParams{
		PublicID: uuid.New(),

		CreatedAt: now,
		UpdatedAt: now,
		ExpiresAt: sql.NullTime{},

		Type:   TypePayment.String(),
		Status: StatusSuccess.String(),

		MerchantID:        mt.ID,
		MerchantOrderUuid: props.MerchantOrderUUID,

		Price:    repository.BigIntToNumeric(price),
		Decimals: int32(decimals),
		Currency: props.Money.Ticker(),

		Description: repository.StringToNullable(props.Description),
		IsTest:      props.IsTest,
		Metadata:    meta.ToJSONB(),
	})

	if err != nil {
		return nil, err
	}

	return s.entryToPayment(pt)
}

func fillPaymentMetaWithLink(meta Metadata, p CreatePaymentProps) Metadata {
	meta[MetaLinkID] = strconv.Itoa(int(p.linkID))
	meta[MetaLinkSuccessAction] = string(p.linkSuccessAction)
	if p.linkSuccessMessage != nil {
		meta[MetaLinkSuccessMessage] = *p.linkSuccessMessage
	}

	return meta
}

type UpdateProps struct {
	Status Status
}

func (s *Service) Update(ctx context.Context, merchantID, id int64, props UpdateProps) (*Payment, error) {
	update := repository.UpdatePaymentParams{
		ID:           id,
		MerchantID:   merchantID,
		UpdatedAt:    time.Now(),
		Status:       props.Status.String(),
		SetExpiresAt: props.Status == StatusLocked,
	}

	if update.SetExpiresAt {
		update.ExpiresAt = repository.TimeToNullable(time.Now().UTC().Add(ExpirationPeriodForLocked))
	}

	pt, err := s.repo.UpdatePayment(ctx, update)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	if pt.MerchantID != 0 {
		evt := bus.PaymentStatusUpdateEvent{
			MerchantID: pt.MerchantID,
			PaymentID:  pt.ID,
		}
		if err := s.publisher.Publish(bus.TopicPaymentStatusUpdate, evt); err != nil {
			return nil, errors.Wrap(err, "unable to publish event")
		}
	}

	return s.entryToPayment(pt)
}

func (s *Service) Fail(ctx context.Context, pt *Payment) error {
	_, err := s.Update(ctx, pt.MerchantID, pt.ID, UpdateProps{Status: StatusFailed})
	return err
}

func (s *Service) SetWebhookTimestamp(ctx context.Context, merchantID, id int64, sentAt time.Time) error {
	err := s.repo.UpdatePaymentWebhookInfo(ctx, repository.UpdatePaymentWebhookInfoParams{
		ID:            id,
		MerchantID:    merchantID,
		WebhookSentAt: repository.TimeToNullable(sentAt),
		UpdatedAt:     time.Now(),
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNotFound
	case err != nil:
		return err
	}

	return nil
}

func (s *Service) GetPaymentMethod(ctx context.Context, p *Payment) (*Method, error) {
	tx, err := s.transactions.GetLatestByPaymentID(ctx, p.ID)

	switch {
	case errors.Is(err, transaction.ErrNotFound):
		return nil, ErrPaymentMethodNotSet
	case err != nil:
		return nil, errors.Wrap(err, "unable to get tx by payment id")
	}

	currency, err := s.blockchain.GetCurrencyByTicker(tx.Currency.Ticker)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get payment currency")
	}

	return MakeMethod(tx, currency), nil
}

func MakeMethod(tx *transaction.Transaction, currency money.CryptoCurrency) *Method {
	return &Method{
		Currency:      currency,
		NetworkID:     tx.NetworkID(),
		IsTest:        tx.IsTest,
		TransactionID: tx.ID,
		tx:            tx,
	}
}

func (s *Service) entryToPayment(p repository.Payment) (*Payment, error) {
	metadata := make(Metadata)
	if p.Metadata.Status == pgtype.Present {
		if err := json.Unmarshal(p.Metadata.Bytes, &metadata); err != nil {
			return nil, err
		}
	}

	price, err := paymentPrice(p, metadata)
	if err != nil {
		return nil, err
	}

	paymentURL := ""
	if p.Type == TypePayment.String() {
		paymentURL = s.paymentURL(p.PublicID)
	}

	entity := &Payment{
		ID:       p.ID,
		PublicID: p.PublicID,

		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
		ExpiresAt: repository.NullTimeToPointer(p.ExpiresAt),

		Type:   Type(p.Type),
		Status: Status(p.Status),

		MerchantID:        p.MerchantID,
		MerchantOrderUUID: p.MerchantOrderUuid,
		MerchantOrderID:   repository.NullableStringToPointer(p.MerchantOrderID),

		Price: price,

		RedirectURL:   p.RedirectUrl,
		PaymentURL:    paymentURL,
		WebhookSentAt: repository.NullTimeToPointer(p.WebhookSentAt),

		Description: repository.NullableStringToPointer(p.Description),
		IsTest:      p.IsTest,

		CustomerID: repository.NullableInt64ToPointer(p.CustomerID),

		metadata: metadata,
	}

	return entity, nil
}

func (s *Service) entriesToPayments(results []repository.Payment) ([]*Payment, error) {
	payments := make([]*Payment, len(results))
	for i := range results {
		p, err := s.entryToPayment(results[i])
		if err != nil {
			return nil, err
		}

		payments[i] = p
	}

	return payments, nil
}

func paymentPrice(p repository.Payment, metadata Metadata) (money.Money, error) {
	decimals := int64(p.Decimals)
	bigInt, err := repository.NumericToBigInt(p.Price)
	if err != nil {
		return money.Money{}, err
	}

	t := Type(p.Type)
	_, isInternal := metadata[MetaInternalPayment]

	switch {
	case t == TypeWithdrawal || (t == TypePayment && isInternal):
		return money.NewFromBigInt(money.Crypto, p.Currency, bigInt, decimals)
	case t == TypePayment:
		currency, err := money.MakeFiatCurrency(p.Currency)
		if err != nil {
			return money.Money{}, err
		}

		return money.NewFromBigInt(money.Fiat, currency.String(), bigInt, decimals)
	}

	return money.Money{}, errors.New("unable to get payment price")
}

func (s *Service) paymentURL(id uuid.UUID) string {
	return fmt.Sprintf("%s/pay/%s", s.basePath, id.String())
}

type CreatePaymentProps struct {
	MerchantOrderUUID uuid.UUID
	MerchantOrderID   *string

	Money money.Money

	RedirectURL *string

	Description *string

	IsTest bool

	// link options
	fromLink           bool
	linkID             int64
	linkSuccessAction  SuccessAction
	linkSuccessMessage *string
}

func (p CreatePaymentProps) validate() error {
	if p.MerchantOrderUUID == uuid.Nil {
		return errors.Wrap(ErrValidation, "merchant order uuid is not set")
	}

	if p.Money.Type() != money.Fiat {
		return errors.Wrap(ErrValidation, "invalid currency")
	}

	float, err := p.Money.FiatToFloat64()
	if err != nil {
		return errors.Wrap(ErrValidation, "invalid price")
	}

	if float <= 0.0 {
		return errors.Wrap(ErrValidation, "price can't be zero or negative")
	}

	if p.RedirectURL != nil {
		if err := validateURL(*p.RedirectURL); err != nil {
			return errors.Wrapf(ErrValidation, "invalid redirect url: %s", err.Error())
		}
	}

	if p.fromLink {
		return p.validateLink()
	}

	return nil
}

func (p CreatePaymentProps) validateLink() error {
	if p.linkID < 1 {
		return errors.Wrap(ErrValidation, "link reference required")
	}

	switch {
	case p.linkSuccessAction == SuccessActionRedirect:
		if p.RedirectURL == nil {
			return errors.Wrap(ErrValidation, "redirectUrl required")
		}
	case p.linkSuccessAction == SuccessActionShowMessage:
		if p.linkSuccessMessage == nil || p.linkSuccessAction == "" {
			return errors.Wrap(ErrValidation, "successMessage required")
		}
	default:
		return errors.Wrapf(ErrValidation, "unknown successAction %q", p.linkSuccessAction)
	}

	return nil
}

func validateURL(u string) error {
	parsed, err := url.ParseRequestURI(u)
	if err != nil {
		return err
	}

	if parsed.Scheme != "https" {
		return errors.New("scheme should be HTTPS")
	}

	if parsed.Hostname() == "" {
		return errors.New("invalid hostname")
	}

	return nil
}

func validateEmail(e string) error {
	if _, err := mail.ParseAddress(e); err != nil {
		return errors.Wrap(ErrValidation, "invalid email")
	}

	return nil
}
