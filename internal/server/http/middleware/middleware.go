package middleware

import (
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo-contrib/session"
	"github.com/labstack/echo/v4"
	mw "github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/oxygenpay/oxygen/internal/server/http/common"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/rs/zerolog"
)

type SessionConfig struct {
	FilesystemPath string `yaml:"path" env:"SESSION_FS_PATH" env-description:"Path to sessions directory"`
	Secret         string `yaml:"secret" env:"SESSION_SECRET" env-description:"Sessions secret, use random string with 8+ characters"`
	CookieDomain   string `yaml:"cookie_domain" env:"SESSION_COOKIE_DOMAIN" env-description:"Cookie domain"`
	CookiePath     string `yaml:"cookie_path" env:"SESSION_COOKIE_PATH" env-default:"/" env-description:"Cookie path"`
	CookieMaxAge   int    `yaml:"cookie_max_age" env:"SESSION_COOKIE_MAX_AGE" env-default:"86400" env-description:"Cookie max-age"` // 60*60*24 = 24h
	CookieSecure   bool   `yaml:"cookie_secure" env:"SESSION_COOKIE_SECURE" env-description:"Allow secure cookies only"`
	CookieHTTPOnly bool   `yaml:"cookie_http_only" env:"SESSION_COOKIE_HTTP_ONLY" env-default:"true" env-description:"Allow HTTP cookies only"`
	CookieSameSite string `yaml:"cookie_same_site" env:"SESSION_COOKIE_SAME_SITE" env-default:"default" env-description:"Cookie same site"`
}

const sessionOptionsKey = "session_options"

const chmodReadWrite = 0660

func Session(cfg SessionConfig) echo.MiddlewareFunc {
	if err := util.EnsureDirectory(cfg.FilesystemPath, chmodReadWrite); err != nil {
		panic(fmt.Sprintf("unable to ensure session directory %q", cfg.FilesystemPath))
	}

	sessionOptions := sessions.Options{
		Path:     cfg.CookiePath,
		Domain:   cfg.CookieDomain,
		MaxAge:   cfg.CookieMaxAge,
		HttpOnly: cfg.CookieHTTPOnly,
		Secure:   cfg.CookieSecure,
		SameSite: parseSameSite(cfg.CookieSameSite),
	}

	applySessionOptions := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set(sessionOptionsKey, sessionOptions)
			return next(c)
		}
	}

	sessionMW := session.Middleware(sessions.NewFilesystemStore(cfg.FilesystemPath, []byte(cfg.Secret)))

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return sessionMW(applySessionOptions(next))
	}
}

// CSRFConfig Cross Site Request Forgery config.
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Set-Cookie
type CSRFConfig struct {
	// CookieDomain Defines the host to which the cookie will be sent.
	// If omitted, this attribute defaults to the host of the current document URL, not including subdomains.
	// Multiple host/domain values are not allowed, but if a domain is specified, then subdomains are always included.
	CookieDomain   string `yaml:"cookie_domain" env:"CSRF_COOKIE_DOMAIN" env-description:"CSRF (Cross Site Request Forgery) cookie domain"`
	CookiePath     string `yaml:"cookie_path" env:"CSRF_COOKIE_PATH" env-default:"/" env-description:"CSRF cookie path"`
	CookieMaxAge   int    `yaml:"cookie_max_age" env:"CSRF_COOKIE_MAX_AGE" env-default:"7200" env-description:"CSRF cookie max-age"` // 2h
	CookieSecure   bool   `yaml:"cookie_secure" env:"CSRF_COOKIE_SECURE" env-description:"CSRF secure cookie"`
	CookieHTTPOnly bool   `yaml:"cookie_http_only" env:"CSRF_COOKIE_HTTP_ONLY" env-default:"true" env-description:"CSRF HTTP only cookie"`
	CookieSameSite string `yaml:"cookie_same_site" env:"CSRF_COOKIE_SAME_SITE" env-default:"default" env-description:"CSRF cookie same site"`
}

func CSRF(cfg CSRFConfig) echo.MiddlewareFunc {
	return mw.CSRFWithConfig(mw.CSRFConfig{
		CookieDomain:   cfg.CookieDomain,
		CookiePath:     cfg.CookiePath,
		CookieHTTPOnly: cfg.CookieHTTPOnly,
		CookieSecure:   cfg.CookieSecure,
		CookieMaxAge:   cfg.CookieMaxAge,
		CookieSameSite: parseSameSite(cfg.CookieSameSite),

		Skipper: func(c echo.Context) bool {
			// if token-based auth -> skip

			tokenAuthRaw := c.Get(IsTokenAuthContextKey)
			isTokenAuth, ok := tokenAuthRaw.(bool)

			return ok && isTokenAuth
		},
	})
}

type CORSConfig struct {
	AllowCredentials bool     `yaml:"allow_credentials" env:"CORS_ALLOW_CREDENTIALS" env-default:"true" env-description:"CORS (Cross Origin Resource Sharing) allow credentials"`
	AllowOrigins     []string `yaml:"allow_origins" env:"CORS_ALLOW_ORIGINS" env-description:"Comma separated list (without spaces) of origins (with schema). Example: https://pay.site.com"`
}

func CORS(cfg CORSConfig) echo.MiddlewareFunc {
	return mw.CORSWithConfig(mw.CORSConfig{
		AllowCredentials: cfg.AllowCredentials,
		AllowOrigins:     cfg.AllowOrigins,
		AllowHeaders: []string{
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderCacheControl,
			CSRFTokenHeader,
		},
	})
}

func Recover(logger *zerolog.Logger) echo.MiddlewareFunc {
	return mw.RecoverWithConfig(mw.RecoverConfig{
		Skipper:           mw.DefaultSkipper,
		StackSize:         4 << 10, // 4 KB
		DisableStackAll:   false,
		DisablePrintStack: false,
		LogLevel:          log.OFF,
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			req := c.Request()

			evt := logger.Error().Err(err)

			evt.Str("stack", string(stack))
			evt.Str("remote_ip", c.RealIP())
			evt.Str("host", req.Host)
			evt.Str("method", req.Method)
			evt.Str("uri", req.RequestURI)
			evt.Str("user_agent", req.UserAgent())
			evt.Str("referer", req.Referer())

			cl := req.Header.Get(echo.HeaderContentLength)
			if cl == "" {
				cl = "0"
			}
			evt.Str("bytes_in", cl)

			evt.Msg("PANIC RECOVERED")

			return common.ErrorResponse(c, "server error")
		},
	})
}

func parseSameSite(v string) http.SameSite {
	switch v {
	case "lax":
		return http.SameSiteLaxMode
	case "none":
		return http.SameSiteNoneMode
	case "strict":
		return http.SameSiteStrictMode
	default:
		return http.SameSiteDefaultMode
	}
}
