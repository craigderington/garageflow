package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"github.com/garageflow/api-go/internal/auth"
	"github.com/garageflow/api-go/internal/config"
	"github.com/garageflow/api-go/internal/customers"
	"github.com/garageflow/api-go/internal/db"
	"github.com/garageflow/api-go/internal/email"
	"github.com/garageflow/api-go/internal/estimates"
	"github.com/garageflow/api-go/internal/events"
	"github.com/garageflow/api-go/internal/inspections"
	"github.com/garageflow/api-go/internal/inventory"
	"github.com/garageflow/api-go/internal/labor"
	"github.com/garageflow/api-go/internal/middleware"
	"github.com/garageflow/api-go/internal/payments"
	"github.com/garageflow/api-go/internal/photos"
	"github.com/garageflow/api-go/internal/portal"
	"github.com/garageflow/api-go/internal/realtime"
	"github.com/garageflow/api-go/internal/repairorders"
	"github.com/garageflow/api-go/internal/scheduling"
	"github.com/garageflow/api-go/internal/sms"
	"github.com/garageflow/api-go/internal/storage"
	"github.com/garageflow/api-go/internal/vehicles"
)

func main() {
	godotenv.Load()
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()
	log.Println("connected to postgres")

	redisOpts, _ := redis.ParseURL(cfg.RedisURL)
	rdb := redis.NewClient(redisOpts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("redis connection failed: %v", err)
	}
	log.Println("connected to redis")

	bus := events.NewBus(rdb)
	hub := realtime.NewHub()

	var mailer email.Sender
	if cfg.MailgunAPIKey != "" && cfg.MailgunDomain != "" {
		mailer = email.NewMailgunSender(cfg.MailgunAPIKey, cfg.MailgunDomain, cfg.MailgunFrom, cfg.MailgunBaseURL)
		log.Printf("email: mailgun sender enabled (domain=%s)", cfg.MailgunDomain)
	} else {
		mailer = email.NewLogSender()
		log.Println("email: mailgun not configured, using no-op log sender")
	}

	authSvc := auth.NewService(pool, rdb, cfg.SessionSecret, mailer, cfg.AppURL)
	authHandler := auth.NewHandler(authSvc, cfg.AuthDevCodes)
	if cfg.AuthDevCodes {
		log.Println("[auth] WARNING: AUTH_DEV_CODES is on — magic-link codes are returned in API responses. Never enable this in production.")
	}

	r := chi.NewRouter()

	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	r.Use(chimw.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Route("/auth", func(r chi.Router) {
		// Credential and magic-link endpoints are unauthenticated and are the
		// obvious brute-force target, so cap attempts per client IP. Disabled
		// (0) for dev and E2E, which log in far faster than any real client.
		if cfg.AuthRateLimitPerMin > 0 {
			r.Use(httprate.LimitByIP(cfg.AuthRateLimitPerMin, time.Minute))
		}

		r.Post("/magic-link", authHandler.MagicLink)
		r.Post("/verify", authHandler.Verify)
		r.Post("/login", authHandler.Login)
		r.Post("/set-password", authHandler.SetPassword)
		r.With(middleware.AuthMiddleware(authSvc)).Get("/me", authHandler.Me)
		r.With(middleware.AuthMiddleware(authSvc)).Post("/logout", authHandler.Logout)
	})

	paySvc := payments.NewService(cfg.StripeSecretKey, cfg.StripeWebhookSecret, cfg.StripeBaseURL)
	if paySvc.Enabled() {
		log.Printf("payments: stripe enabled")
	} else {
		log.Printf("payments: stripe not configured, using dev settlement")
	}
	estHandler := estimates.NewHandler(pool, bus, paySvc, cfg.AppURL)

	store, backend := storage.New(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, false)
	log.Printf("storage: using %s backend", backend)
	photosHandler := photos.NewHandler(pool, store)

	smsSender := sms.New(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioFrom, cfg.TwilioBaseURL)
	if smsSender.Enabled() {
		log.Printf("sms: twilio enabled")
	} else {
		log.Printf("sms: twilio not configured, using no-op log sender")
	}
	inspHandler := inspections.NewHandler(pool, store, smsSender, mailer, cfg.AppURL)

	// Stripe webhook is unauthenticated (verified by signature instead).
	r.Post("/webhooks/stripe", estHandler.Webhook)

	// Public, token-authorized inspection report (no login).
	r.Route("/public/inspections/{token}", func(r chi.Router) {
		r.Get("/", inspHandler.Report)
		r.Get("/photos/{photo_id}", inspHandler.ReportPhoto)
		r.Post("/approve", inspHandler.Approve)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(authSvc))
		r.Use(middleware.TenantMiddleware)

		roHandler := repairorders.NewHandler(pool, bus)
		r.Route("/repair-orders", func(r chi.Router) {
			r.Get("/", roHandler.List)
			r.Post("/", roHandler.Create)
			r.Get("/{id}", roHandler.Get)
			r.Patch("/{id}", roHandler.Update)
			r.Route("/{id}/photos", func(r chi.Router) {
				r.Get("/", photosHandler.List)
				r.Post("/", photosHandler.Upload)
				r.Get("/{photo_id}", photosHandler.Content)
				r.Delete("/{photo_id}", photosHandler.Delete)
			})
			r.Route("/{id}/inspection", func(r chi.Router) {
				r.Post("/", inspHandler.CreateForRO)
				r.Get("/", inspHandler.GetByRO)
			})
		})

		r.Route("/inspections", func(r chi.Router) {
			r.Get("/{id}", inspHandler.Get)
			r.Post("/{id}/send", inspHandler.Send)
			r.Patch("/{id}/items/{item_id}", inspHandler.UpdateItem)
			r.Post("/{id}/items/{item_id}/photos", inspHandler.UploadItemPhoto)
			r.Get("/{id}/photos/{photo_id}", inspHandler.PhotoContent)
			r.Delete("/{id}/photos/{photo_id}", inspHandler.DeletePhoto)
		})

		custHandler := customers.NewHandler(pool)
		r.Route("/customers", func(r chi.Router) {
			r.Get("/", custHandler.List)
			r.Post("/", custHandler.Create)
			r.Get("/{id}", custHandler.Get)
			r.Patch("/{id}", custHandler.Update)
			r.Delete("/{id}", custHandler.Delete)
		})

		vehHandler := vehicles.NewHandler(pool)
		r.Route("/vehicles", func(r chi.Router) {
			r.Get("/", vehHandler.List)
			r.Post("/", vehHandler.Create)
			r.Get("/{id}", vehHandler.Get)
			r.Patch("/{id}", vehHandler.Update)
			r.Delete("/{id}", vehHandler.Delete)
		})

		r.Route("/estimates", func(r chi.Router) {
			r.Post("/", estHandler.Create)
			r.Post("/{id}/send", estHandler.Send)
			r.Post("/{id}/approve", estHandler.Approve)
			r.Post("/{id}/pay", estHandler.Pay)
			r.Get("/ro/{ro_id}", estHandler.GetByRO)
		})

		invHandler := inventory.NewHandler(pool, bus)
		r.Route("/inventory", func(r chi.Router) {
			r.Get("/", invHandler.List)
			r.Post("/", invHandler.Create)
			r.Get("/{id}", invHandler.Get)
			r.Patch("/{id}", invHandler.Update)
			r.Delete("/{id}", invHandler.Delete)
			r.Post("/restock", invHandler.Restock)
		})

		laborHandler := labor.NewHandler(pool, bus)
		r.Route("/labor", func(r chi.Router) {
			r.Post("/clock-in", laborHandler.ClockIn)
			r.Post("/clock-out/{id}", laborHandler.ClockOut)
			r.Get("/ro/{ro_id}", laborHandler.ListByRO)
		})

		schedHandler := scheduling.NewHandler(pool)
		r.Route("/schedule", func(r chi.Router) {
			r.Get("/bays", schedHandler.ListBays)
			r.Post("/bays", schedHandler.CreateBay)
			r.Get("/", schedHandler.ListSchedules)
			r.Post("/", schedHandler.CreateSchedule)
		})

		portalHandler := portal.NewHandler(pool)
		r.Route("/portal", func(r chi.Router) {
			r.Get("/{customer_id}/estimates", portalHandler.GetEstimates)
			r.Get("/{customer_id}/history", portalHandler.GetServiceHistory)
			r.Post("/estimates/{id}/approve", portalHandler.ApproveEstimate)
		})
	})

	r.Get("/ws", hub.ServeWS)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("server starting on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced shutdown: %v", err)
	}
	log.Println("server stopped")
}
