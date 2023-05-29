package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/errors"
)

const (
	Timeout         = time.Second * 5
	HeaderSignature = "X-Signature"
)

var client = http.DefaultClient

var (
	ErrInvalidInput      = errors.New("invalid input")
	ErrInvalidStatusCode = errors.New("invalid status code")
)

func Send(ctx context.Context, destination, secret string, data any) error {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	if err := validateURL(destination); err != nil {
		return errors.Wrap(ErrInvalidInput, err.Error())
	}

	body, err := json.Marshal(data)
	if err != nil {
		return errors.Wrap(ErrInvalidInput, err.Error())
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, destination, bytes.NewReader(body))
	if err != nil {
		return errors.Wrap(ErrInvalidInput, err.Error())
	}

	req.Header.Set("content-type", "application/json")
	if errSign := SignRequest(req, body, secret); errSign != nil {
		return errors.Wrap(ErrInvalidInput, errSign.Error())
	}

	res, err := client.Do(req)
	if err != nil {
		return errors.Wrap(ErrInvalidInput, err.Error())
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return errors.Wrapf(ErrInvalidStatusCode, "code: %d %s", res.StatusCode, res.Status)
	}

	return nil
}

func validateURL(u string) error {
	parsed, err := url.ParseRequestURI(u)
	if err != nil {
		return err
	}

	if parsed.Hostname() == "" {
		return errors.New("invalid hostname")
	}

	return nil
}

func SignRequest(req *http.Request, body []byte, secret string) error {
	if secret == "" {
		return nil
	}

	mac := hmac.New(sha512.New, []byte(secret))
	if _, err := mac.Write(body); err != nil {
		return err
	}

	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	req.Header.Set(HeaderSignature, signature)

	return nil
}
func ValidateHMAC(body []byte, secret, signature string) bool {
	mac := hmac.New(sha512.New, []byte(secret))
	if _, err := mac.Write(body); err != nil {
		return false
	}

	expectedMAC := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return expectedMAC == signature
}
