package log

import (
	"context"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
	echo "github.com/labstack/echo/v4"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/rs/zerolog"
)

type Config struct {
	Level           string `yaml:"level" env:"LOGGER_LEVEL" env-default:"debug" env-description:"Enabled verbose logging"`
	Pretty          bool   `yaml:"pretty" env:"LOGGER_PRETTY" env-default:"false" env-description:"Enables human readable logging. Otherwise, uses json output"`
	SlackWebhookURL string `yaml:"slack_webhook_url" env:"LOGGER_SLACK_WEBHOOK_URL" env-description:"Internal variable"`
}

func New(cfg Config, serviceName, version, env, host string) zerolog.Logger {
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(level)

	out := zerolog.MultiLevelWriter(
		StdoutWriter(cfg.Pretty),
		SlackWriter(cfg.SlackWebhookURL, zerolog.ErrorLevel),
	)

	return zerolog.New(out).
		With().
		Timestamp().
		Str("service", serviceName).
		Str("version", version).
		Str("env", env).
		Str("host", host).
		Caller().
		Logger()
}

// logMarshaler function adapter for zerolog.LogObjectMarshaler
type logMarshaler func(e *zerolog.Event)

func (m logMarshaler) MarshalZerologObject(e *zerolog.Event) {
	m(e)
}

func Ctx(ctx context.Context) zerolog.LogObjectMarshaler {
	return logMarshaler(func(e *zerolog.Event) {
		withRequestID(ctx, e)
	})
}

func withRequestID(ctx context.Context, e *zerolog.Event) {
	id := middleware.RequestIDFromCtx(ctx)
	if id != "" {
		e.Str(middleware.RequestIDKey, id)
	}
}

type requestIDTransport struct {
	transport runtime.ClientTransport
}

// ClientTransport returns OpenAPI client wrapper for propagating request id.
func ClientTransport(origin runtime.ClientTransport) runtime.ClientTransport {
	return &requestIDTransport{origin}
}

func (t *requestIDTransport) Submit(op *runtime.ClientOperation) (any, error) {
	if op.Context == nil {
		return t.transport.Submit(op)
	}

	id := middleware.RequestIDFromCtx(op.Context)
	if id == "" {
		return t.transport.Submit(op)
	}

	params := op.Params

	mw := func(req runtime.ClientRequest, reg strfmt.Registry) error {
		if err := req.SetHeaderParam(echo.HeaderXRequestID, id); err != nil {
			return err
		}

		return params.WriteToRequest(req, reg)
	}

	op.Params = runtime.ClientRequestWriterFunc(mw)

	return t.transport.Submit(op)
}
