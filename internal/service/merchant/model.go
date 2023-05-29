package merchant

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/oxygenpay/oxygen/internal/kms/wallet"
)

type Merchant struct {
	ID        int64
	UUID      uuid.UUID
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	Website   string
	CreatorID int64
	settings  Settings
}

const (
	PropertyWebhookURL      = "webhook.url"
	PropertySignatureSecret = "webhook.secret"
	PropertyPaymentMethods  = "payment.methods"
)

func (m *Merchant) Settings() Settings {
	return m.settings
}

type Property string
type Settings map[Property]string

func (s Settings) WebhookURL() string {
	return s[PropertyWebhookURL]
}

func (s Settings) WebhookSignatureSecret() string {
	return s[PropertySignatureSecret]
}

func (s Settings) PaymentMethods() []string {
	raw := s[PropertyPaymentMethods]
	if raw == "" {
		return nil
	}

	return strings.Split(raw, ",")
}

func (s Settings) toJSONB() pgtype.JSONB {
	if len(s) == 0 {
		return pgtype.JSONB{Status: pgtype.Null}
	}

	raw, _ := json.Marshal(s)

	return pgtype.JSONB{Bytes: raw, Status: pgtype.Present}
}

type Address struct {
	ID             int64
	UUID           uuid.UUID
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Name           string
	MerchantID     int64
	Blockchain     wallet.Blockchain
	BlockchainName string
	Address        string
}
