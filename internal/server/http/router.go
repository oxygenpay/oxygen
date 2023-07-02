package http

import (
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v4"
	mw "github.com/labstack/echo/v4/middleware"
	"github.com/oxygenpay/oxygen/internal/auth"
	v1 "github.com/oxygenpay/oxygen/internal/server/http/internalapi"
	"github.com/oxygenpay/oxygen/internal/server/http/merchantapi"
	merchantauth "github.com/oxygenpay/oxygen/internal/server/http/merchantapi/auth"
	"github.com/oxygenpay/oxygen/internal/server/http/middleware"
	"github.com/oxygenpay/oxygen/internal/server/http/paymentapi"
	"github.com/oxygenpay/oxygen/internal/server/http/webhook"
	"github.com/oxygenpay/oxygen/internal/service/user"
)

// WithDashboardAPI setups routes for Merchant's Dashboard (app.o2pay.co)
func WithDashboardAPI(
	cfg Config,
	handler *merchantapi.Handler,
	authHandler *merchantauth.Handler,
	tokensManager *auth.TokenAuthManager,
	users *user.Service,
	enableEmailAuth bool,
	enableGoogleAuth bool,
) Opt {
	return func(s *Server) {
		guardsUsersMW := middleware.GuardsUsers()

		dashboardAPI := s.echo.Group(
			"/api/dashboard/v1",
			middleware.CORS(cfg.CORS),
			middleware.Session(cfg.Session),
			middleware.ResolvesUserBySession(users),
			middleware.ResolvesUserByToken(tokensManager, users),
			middleware.CSRF(cfg.CSRF),
		)

		authRL := mw.NewRateLimiterMemoryStore(10)
		authGroup := dashboardAPI.Group("/auth", mw.RateLimiter(authRL))

		// common auth routes
		authGroup.GET("/provider", authHandler.ListAvailableProviders)
		authGroup.GET("/csrf-cookie", authHandler.GetCookie)
		authGroup.GET("/me", authHandler.GetMe, guardsUsersMW)
		authGroup.POST("/logout", authHandler.PostLogout, guardsUsersMW)

		// email auth routes
		if enableEmailAuth {
			authGroup.POST("/login", authHandler.PostLogin)
		}

		// google auth routes
		if enableGoogleAuth {
			authGroup.GET("/redirect", authHandler.GetRedirect)
			authGroup.GET("/callback", authHandler.GetCallback)
		}

		dashboardAPI.GET("/merchant", handler.ListMerchants, guardsUsersMW)
		dashboardAPI.POST("/merchant", handler.CreateMerchant, guardsUsersMW)

		// Merchants
		merchantGroup := dashboardAPI.Group(
			"/merchant/:merchantId",
			guardsUsersMW,
			middleware.ResolvesMerchantByUUID(handler.MerchantService()),
			middleware.GuardsMerchants(),
		)

		// Merchant
		merchantGroup.GET("", handler.GetMerchant)
		merchantGroup.PUT("", handler.UpdateMerchant)
		merchantGroup.DELETE("", handler.DeleteMerchant)

		merchantGroup.PUT("/webhook", handler.UpdateMerchantWebhook)
		merchantGroup.PUT("/supported-method", handler.UpdateMerchantSupportedMethods)

		// Merchant Tokens
		merchantGroup.GET("/token", handler.ListMerchantTokens)
		merchantGroup.POST("/token", handler.CreateMerchantToken)
		merchantGroup.DELETE("/token/:tokenId", handler.DeleteMerchantTokens)

		// Merchant Addresses
		merchantGroup.GET("/address", handler.ListMerchantAddresses)
		merchantGroup.POST("/address", handler.CreateMerchantAddress)
		merchantGroup.GET("/address/:addressId", handler.GetMerchantAddress)
		merchantGroup.PUT("/address/:addressId", handler.UpdateMerchantAddress)
		merchantGroup.DELETE("/address/:addressId", handler.DeleteMerchantAddress)

		// Withdrawals
		merchantGroup.POST("/withdrawal", handler.CreateWithdrawal)
		merchantGroup.GET("/withdrawal-fee", handler.GetWithdrawalFee)

		// Form
		merchantGroup.POST("/form", handler.CreateFormSubmission)

		// Currency
		merchantGroup.GET("/currency-convert", handler.GetCurrencyConvert)

		setupCommonMerchantRoutes(merchantGroup, handler)
	}
}

// WithMerchantAPI setups Merchant's API routes (api.o2pay.co)
func WithMerchantAPI(handler *merchantapi.Handler, tokensManager *auth.TokenAuthManager) Opt {
	return func(s *Server) {
		merchantAPI := s.echo.Group(
			"/api/merchant/v1/merchant/:merchantId",
			middleware.ResolvesMerchantByToken(tokensManager, handler.MerchantService()),
			middleware.GuardsMerchants(),
		)

		setupCommonMerchantRoutes(merchantAPI, handler)
	}
}

// setupCommonMerchantRoutes setup shared routes between dashboardAPI and merchantAPI
// session auth: "/api/dashboard/v1/merchant/{merchant}/*"
// token auth: "/api/merchant/v1/merchant/{merchant}/*"
func setupCommonMerchantRoutes(g *echo.Group, handler *merchantapi.Handler) {
	paymentGroup := g.Group("/payment")

	paymentGroup.GET("", handler.ListPayments)
	paymentGroup.GET("/:paymentId", handler.GetPayment)
	paymentGroup.POST("", handler.CreatePayment)

	paymentLinkGroup := g.Group("/payment-link")

	paymentLinkGroup.GET("", handler.ListPaymentLinks)
	paymentLinkGroup.GET("/:paymentLinkId", handler.GetPaymentLink)
	paymentLinkGroup.DELETE("/:paymentLinkId", handler.DeletePaymentLink)
	paymentLinkGroup.POST("", handler.CreatePaymentLink)

	g.GET("/balance", handler.ListBalances)

	g.GET("/customer", handler.ListCustomers)
	g.GET("/customer/:customerId", handler.GetCustomerDetails)
}

// WithPaymentAPI setups routes public-facing payment api (pay.o2pay.co)
func WithPaymentAPI(handler *paymentapi.Handler, cfg Config) Opt {
	return func(s *Server) {
		paymentAPI := s.echo.Group(
			"/api/payment/v1",
			middleware.CORS(cfg.CORS),
			middleware.Session(cfg.Session),
			middleware.CSRF(cfg.CSRF),
		)

		paymentAPI.GET("/csrf-cookie", handler.GetCookie)
		paymentAPI.GET("/currency-convert", handler.GetExchangeRate)

		paymentGroup := paymentAPI.Group(
			"/payment/:paymentId",
			middleware.ResolvesPaymentByPublicID(paymentapi.ParamPaymentID, handler.PaymentService()),
			middleware.GuardsPayment(),
			middleware.RestrictsArchivedPayments(),
		)

		paymentGroup.GET("", handler.GetPayment)
		paymentGroup.PUT("", handler.LockPaymentOptions)
		paymentGroup.POST("/customer", handler.CreateCustomer)
		paymentGroup.POST("/method", handler.CreatePaymentMethod)

		paymentGroup.GET("/supported-method", handler.GetSupportedMethods)

		paymentLinkGroup := paymentAPI.Group("/payment-link")

		paymentRL := mw.NewRateLimiterMemoryStore(1)

		paymentLinkGroup.GET("/:paymentLinkSlug", handler.GetPaymentLink)
		paymentLinkGroup.POST("/:paymentLinkSlug/payment", handler.CreatePaymentFromLink, mw.RateLimiter(paymentRL))
	}
}

func WithWebhookAPI(handler *webhook.Handler) Opt {
	return func(s *Server) {
		webhookAPI := s.echo.Group("/api/webhook/v1")
		webhookAPI.POST("/tatum/:networkId/:walletId", handler.ReceiveTatum)
	}
}

func WithAuthDebug(files fs.FS) Opt {
	return func(s *Server) {
		s.echo.FileFS("/internal/auth-debug", "index.html", files)
	}
}

func WithDocs(files fs.FS) Opt {
	return func(s *Server) {
		s.echo.StaticFS("/internal/docs", files)
	}
}

func WithInternalAPI(h *v1.Handler) Opt {
	return func(s *Server) {
		internal := s.echo.Group("/internal/v1")

		internal.GET("/router", h.GetRouter)

		admin := internal.Group("/admin")
		admin.GET("/wallet", h.ListWallets)
		admin.GET("/wallet/:walletID", h.GetWallet)
		admin.POST("/wallet", h.CreateWallet)
		admin.POST("/wallet/bulk", h.BulkCreateWallets)
		admin.POST("/job", h.RunSchedulerJob)

		admin.POST("/blockchain/fee", h.CalculateTransactionFee)
		admin.POST("/blockchain/broadcast", h.BroadcastTransaction)
		admin.GET("/blockchain/receipt", h.GetTransactionReceipt)
	}
}

const (
	dashboardPrefix = "/dashboard"
	paymentsPrefix  = "/p"
)

func WithEmbeddedFrontend(dashboardUI, paymentsUI fs.FS) Opt {
	return func(s *Server) {
		spaRouter(s.echo, dashboardPrefix, dashboardUI)
		spaRouter(s.echo, paymentsPrefix, paymentsUI)
	}
}

func spaRouter(e *echo.Echo, prefix string, files fs.FS) {
	e.Group(prefix, mw.StaticWithConfig(mw.StaticConfig{
		Root:       "/",
		Index:      "index.html",
		HTML5:      true,
		Filesystem: http.FS(files),
	}))
}
