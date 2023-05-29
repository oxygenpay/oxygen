package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/pkg/errors"
)

type Link struct {
	ID       int64
	PublicID uuid.UUID
	Slug     string
	URL      string

	CreatedAt time.Time
	UpdatedAt time.Time

	MerchantID int64
	Name       string

	Price       money.Money
	Description *string

	SuccessAction  SuccessAction
	RedirectURL    *string
	SuccessMessage *string

	IsTest bool
}

type SuccessAction string

const (
	SuccessActionRedirect    SuccessAction = "redirect"
	SuccessActionShowMessage SuccessAction = "showMessage"
)

type CreateLinkProps struct {
	Name string

	Price       money.Money
	Description *string

	SuccessAction  SuccessAction
	RedirectURL    *string
	SuccessMessage *string

	IsTest bool
}

func (s *Service) ListPaymentLinks(ctx context.Context, merchantID int64) ([]*Link, error) {
	entries, err := s.repo.ListPaymentLinks(ctx, repository.ListPaymentLinksParams{
		MerchantID: merchantID,
		Limit:      100,
	})
	if err != nil {
		return nil, err
	}

	links := make([]*Link, len(entries))

	for i := range entries {
		link, err := s.entryToLink(entries[i])
		if err != nil {
			return nil, err
		}

		links[i] = link
	}

	return links, nil
}

func (s *Service) GetPaymentLinkBySlug(ctx context.Context, slug string) (*Link, error) {
	link, err := s.repo.GetPaymentLinkBySlug(ctx, slug)

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return s.entryToLink(link)
}

func (s *Service) GetPaymentLinkByPublicID(ctx context.Context, merchantID int64, id uuid.UUID) (*Link, error) {
	link, err := s.repo.GetPaymentLinkByPublicID(ctx, repository.GetPaymentLinkByPublicIDParams{
		MerchantID: merchantID,
		Uuid:       id,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return s.entryToLink(link)
}

func (s *Service) GetPaymentLinkByID(ctx context.Context, merchantID, id int64) (*Link, error) {
	link, err := s.repo.GetPaymentLinkByID(ctx, repository.GetPaymentLinkByIDParams{
		MerchantID: merchantID,
		ID:         id,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return s.entryToLink(link)
}

func (s *Service) CreatePaymentLink(ctx context.Context, merchantID int64, props CreateLinkProps) (*Link, error) {
	if err := props.validate(); err != nil {
		return nil, err
	}

	_, err := s.merchants.GetByID(ctx, merchantID, false)
	if err != nil {
		return nil, err
	}

	var description string
	if props.Description != nil {
		description = *props.Description
	}

	link, err := s.repo.CreatePaymentLink(ctx, repository.CreatePaymentLinkParams{
		Uuid:           uuid.New(),
		Slug:           util.Strings.Random(8),
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		MerchantID:     merchantID,
		Name:           props.Name,
		Description:    description,
		Price:          repository.MoneyToNumeric(props.Price),
		Decimals:       int32(props.Price.Decimals()),
		Currency:       props.Price.Ticker(),
		SuccessAction:  string(props.SuccessAction),
		RedirectUrl:    repository.PointerStringToNullable(props.RedirectURL),
		SuccessMessage: repository.PointerStringToNullable(props.SuccessMessage),
		IsTest:         props.IsTest,
	})

	if err != nil {
		return nil, err
	}

	return s.entryToLink(link)
}

func (s *Service) DeletePaymentLinkByPublicID(ctx context.Context, merchantID int64, id uuid.UUID) error {
	if _, err := s.GetPaymentLinkByPublicID(ctx, merchantID, id); err != nil {
		return err
	}

	return s.repo.DeletePaymentLinkByPublicID(ctx, repository.DeletePaymentLinkByPublicIDParams{
		MerchantID: merchantID,
		Uuid:       id,
	})
}

func (s *Service) CreatePaymentFromLink(ctx context.Context, link *Link) (*Payment, error) {
	props := CreatePaymentProps{
		MerchantOrderUUID: uuid.New(),
		Money:             link.Price,
		RedirectURL:       link.RedirectURL,
		Description:       link.Description,
		IsTest:            false,
	}

	return s.CreatePayment(ctx, link.MerchantID, props, FromLink(link))
}

func (p CreateLinkProps) validate() error {
	if p.Name == "" {
		return errors.Wrap(ErrLinkValidation, "name required")
	}

	if p.Price.Type() != money.Fiat {
		return errors.Wrap(ErrLinkValidation, "invalid currency")
	}

	float, err := p.Price.FiatToFloat64()
	if err != nil {
		return errors.Wrap(ErrLinkValidation, "invalid price")
	}

	if float <= 0.0 {
		return errors.Wrap(ErrLinkValidation, "price can't be zero or negative")
	}

	switch p.SuccessAction {
	case SuccessActionRedirect:
		if p.RedirectURL == nil {
			return errors.Wrap(ErrLinkValidation, "redirectUrl required")
		}
		if err := validateURL(*p.RedirectURL); err != nil {
			return errors.Wrapf(ErrLinkValidation, "invalid redirect url: %s", err.Error())
		}
		if p.SuccessMessage != nil {
			return errors.Wrap(ErrLinkValidation, "successMessage should not be present")
		}
	case SuccessActionShowMessage:
		if p.SuccessMessage == nil || *p.SuccessMessage == "" {
			return errors.Wrapf(ErrLinkValidation, "successMessage required")
		}
		if p.RedirectURL != nil {
			return errors.Wrap(ErrLinkValidation, "redirectUrl should not be present")
		}
	default:
		return errors.Wrap(ErrLinkValidation, "invalid successAction")
	}

	return nil
}

func (s *Service) linkURL(slug string) string {
	return fmt.Sprintf("%s/link/%s", s.basePath, slug)
}

func (s *Service) entryToLink(link repository.PaymentLink) (*Link, error) {
	bigInt, err := repository.NumericToBigInt(link.Price)
	if err != nil {
		return nil, err
	}

	currency, err := money.MakeFiatCurrency(link.Currency)
	if err != nil {
		return nil, err
	}

	price, err := money.NewFromBigInt(money.Fiat, currency.String(), bigInt, int64(link.Decimals))
	if err != nil {
		return nil, err
	}

	var desc *string
	if link.Description != "" {
		desc = &link.Description
	}

	return &Link{
		ID:       link.ID,
		PublicID: link.Uuid,
		Slug:     link.Slug,
		URL:      s.linkURL(link.Slug),

		CreatedAt: link.CreatedAt,
		UpdatedAt: link.UpdatedAt,

		MerchantID: link.MerchantID,
		Name:       link.Name,

		Price:       price,
		Description: desc,

		SuccessAction:  SuccessAction(link.SuccessAction),
		RedirectURL:    repository.NullableStringToPointer(link.RedirectUrl),
		SuccessMessage: repository.NullableStringToPointer(link.SuccessMessage),

		IsTest: link.IsTest,
	}, nil
}
