package middleware

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/service/payment"
	"github.com/pkg/errors"
)

const PaymentContextKey = "payment"

type PaymentResolver interface {
	GetByPublicID(ctx context.Context, publicID uuid.UUID) (*payment.Payment, error)
}

// ResolvesUserByToken attaches user to echo.Context
// if user still isn't set by session
func ResolvesPaymentByPublicID(paramName string, payments PaymentResolver) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			id, err := common.UUID(c, paramName)
			if err != nil {
				return err
			}

			ctx := c.Request().Context()

			p, err := payments.GetByPublicID(ctx, id)

			switch {
			case errors.Is(err, payment.ErrNotFound):
				return next(c)
			case err != nil:
				return errors.Wrap(err, "unable to get payment")
			}

			c.Set(PaymentContextKey, p)

			return next(c)
		}
	}
}

func GuardsPayment() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if _, err := ResolvePayment(c); err != nil {
				return common.NotFoundResponse(c, "payment not found")
			}

			return next(c)
		}
	}
}

func ResolvePayment(c echo.Context) (*payment.Payment, error) {
	raw := c.Get(PaymentContextKey)
	p, ok := raw.(*payment.Payment)
	if !ok || p == nil || p.ID == 0 {
		return nil, errors.New("unable to resolve payment")
	}

	return p, nil
}

// RestrictsArchivedPayments restricts user from accessing successful/failed payments
// after certain time window.
func RestrictsArchivedPayments() echo.MiddlewareFunc {
	const window = time.Minute * 60

	restrict := func(pt *payment.Payment) bool {
		if pt.Status != payment.StatusSuccess && pt.Status != payment.StatusFailed {
			return false
		}

		return time.Since(pt.UpdatedAt) > window
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if pt, err := ResolvePayment(c); err == nil {
				if restrict(pt) {
					return common.NotFoundResponse(c, "payment is archived")
				}
			}

			return next(c)
		}
	}
}
