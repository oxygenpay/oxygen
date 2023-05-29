package payment

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v4"
	"github.com/oxygenpay/oxygen/internal/db/repository"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/pkg/errors"
)

func (s *Service) GetCustomerByEmail(ctx context.Context, merchantID int64, email string) (*Customer, error) {
	entry, err := s.repo.GetCustomerByEmail(ctx, repository.GetCustomerByEmailParams{
		MerchantID: merchantID,
		Email:      repository.StringToNullable(email),
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return entryToCustomer(entry)
}

func (s *Service) GetCustomerByID(ctx context.Context, merchantID, id int64) (*Customer, error) {
	entry, err := s.repo.GetCustomerByID(ctx, repository.GetCustomerByIDParams{
		ID:         id,
		MerchantID: merchantID,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return entryToCustomer(entry)
}

func (s *Service) GetBatchCustomers(ctx context.Context, merchantID int64, ids []int64) ([]*Customer, error) {
	entries, err := s.repo.GetBatchCustomers(ctx, repository.GetBatchCustomersParams{
		MerchantID: merchantID,
		Ids:        util.MapSlice(ids, func(i int64) int32 { return int32(i) }),
	})
	if err != nil {
		return nil, err
	}

	customers, err := entriesToCustomers(entries)
	if err != nil {
		return nil, errors.Wrap(err, "unable to map customers")
	}

	return customers, nil
}

func (s *Service) GetCustomerByUUID(ctx context.Context, merchantID int64, id uuid.UUID) (*Customer, error) {
	c, err := s.repo.GetCustomerByUUID(ctx, repository.GetCustomerByUUIDParams{
		MerchantID: merchantID,
		Uuid:       id,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, err
	}

	return entryToCustomer(c)
}

func (s *Service) GetCustomerDetailsByUUID(ctx context.Context, merchantID int64, id uuid.UUID) (*CustomerDetails, error) {
	c, err := s.GetCustomerByUUID(ctx, merchantID, id)
	if err != nil {
		return nil, err
	}

	successfulPayments, _ := s.repo.CalculateCustomerPayments(ctx, repository.CalculateCustomerPaymentsParams{
		MerchantID: merchantID,
		CustomerID: repository.Int64ToNullable(c.ID),
		Status:     StatusSuccess.String(),
	})

	entries, err := s.repo.GetRecentCustomerPayments(ctx, repository.GetRecentCustomerPaymentsParams{
		MerchantID: merchantID,
		CustomerID: repository.Int64ToNullable(c.ID),
		Limit:      10,
	})
	if err != nil {
		return nil, errors.Wrap(err, "unable to list recent customer payments")
	}

	payments, err := s.entriesToPayments(entries)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get payments")
	}

	return &CustomerDetails{
		Customer:           *c,
		SuccessfulPayments: successfulPayments,
		RecentPayments:     payments,
	}, nil
}

func (s *Service) ListCustomers(ctx context.Context, merchantID int64, opts ListOptions) ([]*Customer, string, error) {
	// 1. setup limit
	limit := int32(opts.Limit)
	if limit == 0 {
		limit = limitDefault
	}

	if limit > limitMax {
		return nil, "", ErrInvalidLimit
	}

	// 2. resolve cursor. If cursor provided, then we need to map cursor to customer.id
	var cursorCustomer *Customer
	if opts.Cursor != "" {
		customerID, err := uuid.Parse(opts.Cursor)
		if err != nil {
			return nil, "", errors.Wrap(ErrValidation, "invalid cursor")
		}

		c, err := s.GetCustomerByUUID(ctx, merchantID, customerID)
		if err != nil {
			return nil, "", errors.Wrap(err, "unable to get fromID customer")
		}

		cursorCustomer = c
	}

	var results []repository.Customer
	var err error

	if opts.ReverseOrder {
		fromID := int64(math.MaxInt64)
		if cursorCustomer != nil {
			fromID = cursorCustomer.ID
		}

		results, err = s.repo.PaginateCustomersDesc(ctx, repository.PaginateCustomersDescParams{
			MerchantID: merchantID,
			ID:         fromID,
			Limit:      limit + 1,
		})
	} else {
		var fromID int64
		if cursorCustomer != nil {
			fromID = cursorCustomer.ID
		}

		results, err = s.repo.PaginateCustomersAsc(ctx, repository.PaginateCustomersAscParams{
			MerchantID: merchantID,
			ID:         fromID,
			Limit:      limit + 1,
		})
	}

	if err != nil {
		return nil, "", errors.Wrap(err, "unable to paginate customers")
	}

	// 3. map results
	customers, err := entriesToCustomers(results)
	if err != nil {
		return nil, "", errors.Wrap(err, "unable to map customers")
	}

	// 4. in case of 'limit + 1' entries last item is
	// next cursor => resolve nextCursor and remove it from results.
	var nextCursor string
	if len(customers) > int(limit) {
		nextCursor = customers[len(customers)-1].UUID.String()
		customers = customers[:limit]
	}

	return customers, nextCursor, nil
}

func (s *Service) CreateCustomer(ctx context.Context, merchantID int64, email string) (*Customer, error) {
	entry, err := s.repo.CreateCustomer(ctx, repository.CreateCustomerParams{
		MerchantID: merchantID,
		Uuid:       uuid.New(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Email:      repository.StringToNullable(email),
	})

	if err != nil {
		return nil, err
	}

	return entryToCustomer(entry)
}

// ResolveCustomerByEmail fetches Customer from DB or creates it on-the-fly.
func (s *Service) ResolveCustomerByEmail(ctx context.Context, merchantID int64, email string) (*Customer, error) {
	entry, err := s.repo.GetCustomerByEmail(ctx, repository.GetCustomerByEmailParams{
		Email:      repository.StringToNullable(email),
		MerchantID: merchantID,
	})

	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return s.CreateCustomer(ctx, merchantID, email)
	case err != nil:
		return nil, err
	}

	return entryToCustomer(entry)
}

// AssignCustomerByEmail resolves customer by email and then attaches him to the current payment.
func (s *Service) AssignCustomerByEmail(ctx context.Context, p *Payment, email string) (*Customer, error) {
	if err := validateEmail(email); err != nil {
		return nil, err
	}

	if p.Status != StatusPending {
		return nil, ErrPaymentLocked
	}

	person, err := s.ResolveCustomerByEmail(ctx, p.MerchantID, email)
	if err != nil {
		return nil, errors.Wrap(err, "unable to resolve customer by email")
	}

	err = s.repo.UpdatePaymentCustomerID(ctx, repository.UpdatePaymentCustomerIDParams{
		ID:         p.ID,
		CustomerID: repository.Int64ToNullable(person.ID),
	})

	if err != nil {
		return nil, errors.Wrap(err, "unable to update payment customer id")
	}

	return person, nil
}

func entryToCustomer(c repository.Customer) (*Customer, error) {
	return &Customer{
		ID:         c.ID,
		MerchantID: c.MerchantID,
		UUID:       c.Uuid,
		CreatedAt:  c.CreatedAt,
		UpdatedAt:  c.UpdatedAt,
		Email:      c.Email.String,
	}, nil
}

func entriesToCustomers(results []repository.Customer) ([]*Customer, error) {
	payments := make([]*Customer, len(results))
	for i := range results {
		p, err := entryToCustomer(results[i])
		if err != nil {
			return nil, err
		}

		payments[i] = p
	}

	return payments, nil
}
