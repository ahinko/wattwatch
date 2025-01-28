// Package routes handles the setup and configuration of API routes
package routes

import (
	"database/sql"
	_ "wattwatch/docs" // Import swagger docs
	"wattwatch/internal/api/handlers"
	"wattwatch/internal/api/middleware"
	"wattwatch/internal/auth"
	"wattwatch/internal/config"
	"wattwatch/internal/email"
	"wattwatch/internal/provider"
	"wattwatch/internal/repository/postgres"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// SetupRoutes configures all API routes and their handlers
func SetupRoutes(cfg *config.Config, db *sql.DB, providerManager *provider.Manager) *gin.Engine {
	// Create router
	r := gin.Default()

	// Apply compression middleware globally
	r.Use(middleware.Compression(middleware.DefaultCompressionConfig()))

	// Initialize health handler for basic routes
	healthHandler := handlers.NewHealthHandler(db)

	// Routes without rate limiting
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Apply rate limiting to all other routes
	r.Use(middleware.NewRateLimiter(cfg).Middleware())

	// Add provider manager to context
	r.Use(func(c *gin.Context) {
		c.Set("providerManager", providerManager)
		c.Next()
	})

	// Initialize repositories
	passwordHistory := postgres.NewPasswordHistoryRepository(db)
	userRepo := postgres.NewUserRepository(db)
	roleRepo := postgres.NewRoleRepository(db)
	auditRepo := postgres.NewAuditLogRepository(db)
	refreshTokenRepo := postgres.NewRefreshTokenRepository(db)
	currencyRepo := postgres.NewCurrencyRepository(db)
	zoneRepo := postgres.NewZoneRepository(db)
	spotPriceRepo := postgres.NewSpotPriceRepository(db)
	loginAttemptRepo := postgres.NewLoginAttemptRepository(db)
	emailVerifyRepo := postgres.NewEmailVerificationRepository(db)
	passwordResetRepo := postgres.NewPasswordResetRepository(db)

	// Initialize services
	authService := auth.NewService(cfg, refreshTokenRepo)
	emailService := email.NewService(cfg.Email)

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(authService, userRepo, roleRepo)

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(
		userRepo,
		roleRepo,
		authService,
		auditRepo,
		emailService,
		cfg,
		loginAttemptRepo,
		emailVerifyRepo,
		passwordResetRepo,
	)
	userHandler := handlers.NewUserHandler(userRepo, authService, passwordHistory, auditRepo)
	roleHandler := handlers.NewRoleHandler(roleRepo, userRepo, auditRepo)
	currencyHandler := handlers.NewCurrencyHandler(currencyRepo)
	zoneHandler := handlers.NewZoneHandler(zoneRepo)
	spotPriceHandler := handlers.NewSpotPriceHandler(spotPriceRepo, zoneRepo, currencyRepo)
	providerHandler := handlers.NewProviderHandler(providerManager)

	// API v1 routes
	v1 := r.Group("/api/v1")
	{
		// Health check (no authentication required)
		v1.GET("/health", healthHandler.Health)

		// Auth routes
		auth := v1.Group("/auth")
		{
			auth.POST("/login", authHandler.Login)
			auth.POST("/register", authHandler.Register)
			auth.GET("/verify-email", authHandler.VerifyEmail)
			auth.POST("/resend-verification", authMiddleware.AuthRequired(), authHandler.ResendVerification)
			auth.POST("/reset-password", authHandler.RequestPasswordReset)
			auth.POST("/reset-password/complete", authHandler.CompletePasswordReset)
			auth.POST("/refresh", authHandler.Refresh)
		}

		// User routes (requires authentication)
		users := v1.Group("/users")
		users.Use(authMiddleware.AuthRequired())
		{
			users.GET("", userHandler.ListUsers)
			users.GET("/:id", userHandler.GetUser)
			users.PUT("/:id", userHandler.UpdateUser)
			users.PUT("/:id/password", userHandler.ChangePassword)
			users.DELETE("/:id", userHandler.DeleteUser)
		}

		// Role routes (requires authentication)
		roles := v1.Group("/roles")
		roles.Use(authMiddleware.AuthRequired())
		{
			roles.GET("", roleHandler.ListRoles)
			roles.GET("/:id", roleHandler.GetRole)
			roles.POST("", roleHandler.CreateRole)
			roles.PUT("/:id", roleHandler.UpdateRole)
			roles.DELETE("/:id", roleHandler.DeleteRole)
		}

		// Currency routes
		currencies := v1.Group("/currencies")
		{
			// Public routes (require authentication)
			currencies.Use(authMiddleware.AuthRequired())
			currencies.GET("", currencyHandler.ListCurrencies)
			currencies.GET("/:id", currencyHandler.GetCurrency)

			// Admin-only routes
			adminCurrencies := currencies.Group("")
			adminCurrencies.Use(authMiddleware.AdminRequired())
			{
				adminCurrencies.POST("", currencyHandler.CreateCurrency)
				adminCurrencies.PUT("/:id", currencyHandler.UpdateCurrency)
				adminCurrencies.DELETE("/:id", currencyHandler.DeleteCurrency)
			}
		}

		// Zone routes
		zones := v1.Group("/zones")
		{
			// Public routes (require authentication)
			zones.Use(authMiddleware.AuthRequired())
			zones.GET("", zoneHandler.ListZones)
			zones.GET("/:id", zoneHandler.GetZone)

			// Admin-only routes
			adminZones := zones.Group("")
			adminZones.Use(authMiddleware.AdminRequired())
			{
				adminZones.POST("", zoneHandler.CreateZone)
				adminZones.PUT("/:id", zoneHandler.UpdateZone)
				adminZones.DELETE("/:id", zoneHandler.DeleteZone)
			}
		}

		// Spot price routes
		spotPrices := v1.Group("/spot-prices")
		{
			spotPrices.GET("", spotPriceHandler.ListSpotPrices)
			spotPrices.GET("/:id", spotPriceHandler.GetSpotPrice)
			spotPrices.POST("", authMiddleware.AdminRequired(), spotPriceHandler.CreateSpotPrices)
			spotPrices.DELETE("/:id", authMiddleware.AdminRequired(), spotPriceHandler.DeleteSpotPrice)
		}

		// Provider routes
		providers := v1.Group("/providers")
		providers.Use(authMiddleware.AdminRequired())
		{
			providers.POST("/nordpool/fetch", providerHandler.TriggerNordpoolFetch)
		}
	}

	return r
}
