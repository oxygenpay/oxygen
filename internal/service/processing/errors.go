package processing

import "github.com/pkg/errors"

var (
	ErrStatusInvalid         = errors.New("payment status is invalid")
	ErrPaymentOptionsMissing = errors.New("payment options are not fully fulfilled")
	ErrSignatureVerification = errors.New("unable to verify request signature")
	ErrInboundWallet         = errors.New("inbound wallet error")
)

type CancellableError struct {
	err error
}

func (e *CancellableError) Error() string {
	return e.err.Error()
}

func (e *CancellableError) Unwrap() error {
	return e.err
}
