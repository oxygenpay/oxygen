package http

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/rs/zerolog"
	"github.com/ziflex/lecho/v3"
)

type Config struct {
	Address string `yaml:"address" env:"WEB_ADDRESS" env-default:"0.0.0.0" env-description:"Listen address"`
	Port    string `yaml:"port" env:"WEB_PORT" env-default:"80" env-description:"Listen port"`

	Session middleware.SessionConfig `yaml:"session"`
	CSRF    middleware.CSRFConfig    `yaml:"csrf"`
	CORS    middleware.CORSConfig    `yaml:"cors"`

	EnableInternalAPI bool `yaml:"enable_internal_api" env:"WEB_ENABLE_INTERNAL_API" env-default:"false" env-description:"Enables internal API /internal/v1/*. DO NOT EXPOSE TO PUBLIC"`
}

type Server struct {
	echo        *echo.Echo
	address     string
	logger      *zerolog.Logger
	logRequests bool
}

type Opt func(s *Server)

func NoOpt() Opt {
	return func(_ *Server) {}
}

func New(cfg Config, logRequests bool, opts ...Opt) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	srv := &Server{
		echo:        e,
		address:     cfg.Address + ":" + cfg.Port,
		logRequests: logRequests,
	}

	// obligatory middlewares
	e.Use(middleware.RequestID())
	withHealthcheck(e)

	// user-defined middlewares
	for _, option := range opts {
		option(srv)
	}

	return srv
}

func WithRecover() Opt {
	return func(s *Server) {
		s.echo.Use(middleware.Recover(s.logger))
	}
}

func WithLogger(logger *zerolog.Logger) Opt {
	return func(s *Server) {
		l := logger.With().Str("channel", "web_server").Logger()
		s.logger = &l

		if !s.logRequests {
			s.echo.Logger = lecho.From(zerolog.Nop())
			return
		}

		s.echo.Use(lecho.Middleware(lecho.Config{
			Logger:       lecho.From(l, lecho.WithLevel(log.INFO)),
			RequestIDKey: middleware.RequestIDKey,
			Skipper: func(c echo.Context) bool {
				return c.Request().URL.Path == healthcheckPath
			},
		}))
	}
}

const healthcheckPath = "/health"

func withHealthcheck(e *echo.Echo) {
	e.GET(healthcheckPath, func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})
}

func WithBodyDump() Opt {
	return func(s *Server) {
		s.echo.Use(middleware.BodyDump())
	}
}

func (s *Server) Echo() *echo.Echo {
	return s.echo
}

func (s *Server) Run() error {
	return s.echo.Start(s.address)
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.echo.Shutdown(ctx)
}

func (s *Server) Address() string {
	return s.address
}
