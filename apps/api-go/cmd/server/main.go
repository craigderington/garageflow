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
	"github.com/garageflow/api-go/internal/demo"
	"github.com/garageflow/api-go/internal/email"
	"github.com/garageflow/api-go/internal/estimates"
	"github.com/garageflow/api-go/internal/events"
	"github.com/garageflow/api-go/internal/health"
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
	"github.com/garageflow/api-go/internal/users"
	"github.com/garageflow/api-go/internal/vehicles"
)

// requestTimeout bounds ordinary request handling. Applied per route group
// rather than globally, because the SSE stream at /events must not inherit it.
const requestTimeout = 30 * time.Second

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

	// Long-lived, unlike the startup ctx above which carries a 10s deadline.
	hub := realtime.NewHub()
	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()
	hub.SubscribeShopEvents(hubCtx, rdb)

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
	// Order matters: capturePeerIP must run BEFORE RealIP, which overwrites
	// r.RemoteAddr from client-supplied headers. See ratelimit.go — the demo
	// and auth limiters key on the peer it stashes, not on the header.
	r.Use(capturePeerIP)
	r.Use(chimw.RealIP)
	r.Use(chimw.RequestID)
	// NOTE: no global chimw.Timeout. It sets a deadline on the request context,
	// which would cancel the SSE stream at /events after 30s. Applied per route
	// group below instead, so everything except the stream still gets it.
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:5173", cfg.AppURL},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Unauthenticated and deliberately outside chimw.Timeout — the handler
	// bounds itself, and a monitor must be able to reach this when the app is
	// too unhealthy to serve anything else.
	r.Get("/healthz", health.New(
		health.Check{Name: "postgres", Ping: pool.Ping},
		health.Check{Name: "redis", Ping: func(ctx context.Context) error { return rdb.Ping(ctx).Err() }},
	).ServeHTTP)

	demoSvc := demo.NewService(pool, authSvc, 14*24*time.Hour)
	demoHandler := demo.NewHandler(demoSvc, mailer, cfg.AppURL)

	r.Route("/demo", func(r chi.Router) {
		r.Use(chimw.Timeout(requestTimeout))
		// keyByTrustedIP, not httprate.LimitByIP: LimitByIP keys on
		// r.RemoteAddr, which chimw.RealIP has already overwritten from
		// True-Client-IP / X-Real-IP / X-Forwarded-For, so the limit would be
		// bypassable by rotating one header — on the one endpoint where each
		// request costs a bcrypt hash, ~25 rows that live 14 days, and an
		// unauthenticated email to a caller-chosen address.
		if cfg.DemoRateLimitPerMin > 0 {
			r.Use(httprate.Limit(cfg.DemoRateLimitPerMin, time.Minute, httprate.WithKeyFuncs(keyByTrustedIP)))
		}
		r.Post("/", demoHandler.Start)
		r.Post("/resume", demoHandler.Resume)
	})

	go func() {
		sweep := func() {
			if n, err := demoSvc.SweepExpired(hubCtx); err != nil {
				log.Printf("[demo] sweep failed: %v", err)
			} else if n > 0 {
				log.Printf("[demo] swept %d expired demo shops", n)
			}
		}
		// Sweep once on boot, before waiting out the first tick. Without this,
		// a deploy cadence faster than the tick means expired shops are never
		// collected — every restart resets the timer.
		sweep()

		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-hubCtx.Done():
				return
			case <-ticker.C:
				sweep()
			}
		}
	}()

	r.Route("/auth", func(r chi.Router) {
		r.Use(chimw.Timeout(requestTimeout))
		// Credential and magic-link endpoints are unauthenticated and are the
		// obvious brute-force target, so cap attempts per client IP. Disabled
		// (0) for dev and E2E, which log in far faster than any real client.
		// Same header-spoofing-resistant key as /demo above.
		if cfg.AuthRateLimitPerMin > 0 {
			r.Use(httprate.Limit(cfg.AuthRateLimitPerMin, time.Minute, httprate.WithKeyFuncs(keyByTrustedIP)))
		}

		r.Post("/magic-link", authHandler.MagicLink)
		r.Post("/verify", authHandler.Verify)
		r.Post("/login", authHandler.Login)
		r.With(middleware.AuthMiddleware(authSvc)).Post("/set-password", authHandler.SetPassword)
		r.With(middleware.AuthMiddleware(authSvc)).Get("/me", authHandler.Me)
		r.With(middleware.AuthMiddleware(authSvc)).Post("/logout", authHandler.Logout)
	})

	paySvc := payments.NewService(cfg.StripeSecretKey, cfg.StripeWebhookSecret, cfg.StripeBaseURL)
	if paySvc.Enabled() {
		log.Printf("payments: stripe enabled")
	} else {
		log.Printf("payments: stripe not configured, using dev settlement")
	}
	// demo.IsDemoShop keeps a prospect's "collect payment" click off the
	// merchant's real Stripe account — the payment counterpart to the SMS and
	// email guards wired up below.
	estHandler := estimates.NewHandler(pool, bus, paySvc, cfg.AppURL, func(ctx context.Context) bool {
		return demo.IsDemoShop(ctx, pool)
	})

	store, backend := storage.New(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOBucket, false)
	log.Printf("storage: using %s backend", backend)
	photosHandler := photos.NewHandler(pool, store)

	smsSender := sms.New(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioFrom, cfg.TwilioBaseURL)
	if smsSender.Enabled() {
		log.Printf("sms: twilio enabled")
	} else {
		log.Printf("sms: twilio not configured, using no-op log sender")
	}

	// Demo tenants must never send to a real phone or inbox. Wrap once, here,
	// so no future handler can accidentally take the unguarded sender.
	guardedSMS := demo.GuardSMS(smsSender, pool)
	guardedMailer := demo.GuardEmail(mailer, pool)

	inspHandler := inspections.NewHandler(pool, store, guardedSMS, guardedMailer, cfg.AppURL)

	// Stripe webhook is unauthenticated (verified by signature instead).
	r.With(chimw.Timeout(requestTimeout)).Post("/webhooks/stripe", estHandler.Webhook)

	// Public, token-authorized inspection report (no login).
	r.Route("/public/inspections/{token}", func(r chi.Router) {
		r.Use(chimw.Timeout(requestTimeout))
		r.Get("/", inspHandler.Report)
		r.Get("/photos/{photo_id}", inspHandler.ReportPhoto)
		r.Post("/approve", inspHandler.Approve)
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(authSvc))
		r.Use(middleware.TenantMiddleware)
		r.Use(chimw.Timeout(requestTimeout))

		roHandler := repairorders.NewHandler(pool, bus)
		r.Route("/repair-orders", func(r chi.Router) {
			r.Get("/", roHandler.List)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/", roHandler.Create)
			r.Get("/{id}", roHandler.Get)
			r.Patch("/{id}", roHandler.Update)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Delete("/{id}", roHandler.Delete)
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
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/", custHandler.Create)
			r.Get("/{id}", custHandler.Get)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Patch("/{id}", custHandler.Update)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Put("/{id}", custHandler.Update)
			r.With(middleware.RBACMiddleware("owner", "admin")).Delete("/{id}", custHandler.Delete)
		})

		vehHandler := vehicles.NewHandler(pool)
		r.Route("/vehicles", func(r chi.Router) {
			r.Get("/", vehHandler.List)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/", vehHandler.Create)
			r.Get("/{id}", vehHandler.Get)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Patch("/{id}", vehHandler.Update)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Put("/{id}", vehHandler.Update)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/{id}/archive", vehHandler.Archive)
			r.With(middleware.RBACMiddleware("owner", "admin")).Delete("/{id}", vehHandler.Delete)
		})

		r.Route("/estimates", func(r chi.Router) {
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/", estHandler.Create)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Put("/{id}", estHandler.Update)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Patch("/{id}", estHandler.Update)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/{id}/send", estHandler.Send)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/{id}/approve", estHandler.Approve)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/{id}/pay", estHandler.Pay)
			r.Get("/ro/{ro_id}", estHandler.GetByRO)
		})

		userHandler := users.NewHandler(pool)
		r.Route("/users", func(r chi.Router) {
			r.Get("/", userHandler.List)
			r.With(middleware.RBACMiddleware("owner", "admin")).Post("/", userHandler.Create)
			r.With(middleware.RBACMiddleware("owner", "admin")).Put("/{id}", userHandler.Update)
			r.With(middleware.RBACMiddleware("owner", "admin")).Patch("/{id}", userHandler.Update)
			r.With(middleware.RBACMiddleware("owner", "admin")).Delete("/{id}", userHandler.Delete)
		})

		invHandler := inventory.NewHandler(pool, bus)
		r.Route("/inventory", func(r chi.Router) {
			r.Get("/", invHandler.List)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/", invHandler.Create)
			r.Get("/{id}", invHandler.Get)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Patch("/{id}", invHandler.Update)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/{id}/archive", invHandler.Archive)
			r.With(middleware.RBACMiddleware("owner", "admin")).Delete("/{id}", invHandler.Delete)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/restock", invHandler.Restock)
		})

		laborHandler := labor.NewHandler(pool, bus)
		r.Route("/labor", func(r chi.Router) {
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/manual", laborHandler.AddManual)
			r.Post("/clock-in", laborHandler.ClockIn)
			r.Post("/clock-out/{id}", laborHandler.ClockOut)
			r.Get("/ro/{ro_id}", laborHandler.ListByRO)
		})

		schedHandler := scheduling.NewHandler(pool)
		r.Route("/schedule", func(r chi.Router) {
			r.Get("/bays", schedHandler.ListBays)
			r.With(middleware.RBACMiddleware("owner", "admin")).Post("/bays", schedHandler.CreateBay)
			r.With(middleware.RBACMiddleware("owner", "admin")).Delete("/bays/{id}", schedHandler.DeleteBay)
			r.Get("/", schedHandler.ListSchedules)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/", schedHandler.CreateSchedule)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/assign", schedHandler.AssignRO)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/unassign", schedHandler.UnassignRO)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Delete("/{id}", schedHandler.DeleteSchedule)
		})

		portalHandler := portal.NewHandler(pool)
		r.Route("/portal", func(r chi.Router) {
			r.Get("/{customer_id}/estimates", portalHandler.GetEstimates)
			r.Get("/{customer_id}/history", portalHandler.GetServiceHistory)
			r.With(middleware.RBACMiddleware("owner", "admin", "service_writer")).Post("/estimates/{id}/approve", portalHandler.ApproveEstimate)
		})
	})

	// Server-Sent Events. Authenticated and tenant-scoped like the group above,
	// but deliberately without chimw.Timeout: a stream is meant to stay open,
	// and a request deadline would cut it off mid-flight. The handler clears the
	// server's WriteTimeout for the same reason.
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(authSvc))
		r.Use(middleware.TenantMiddleware)

		r.Get("/events", hub.ServeEvents)
	})

	srv := &http.Server{
		Addr:        ":" + cfg.Port,
		Handler:     r,
		ReadTimeout: 15 * time.Second,
		// WriteTimeout stays off: it cannot be cleared per-request for streams
		// in a way that works across all response wrappers, and the SSE handler
		// needs an unbounded write window. ReadTimeout and IdleTimeout still
		// bound slow-loris style connections.
		WriteTimeout: 0,
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
