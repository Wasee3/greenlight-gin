package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"os/signal"

	"github.com/Nerzal/gocloak/v13"
	"github.com/Wasee3/greenlight-gin/internal/data"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
)

const version = "1.0.0"

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  time.Duration
	}
	kc struct {
		Realm          string
		AuthURL        string
		admin_username string
		admin_password string
		client_id      string
		client_secret  string
		kc_jwks_url    string
		kc_issuer_url  string
	}
	cors struct {
		trustedOrigins []string
	}
}

type application struct {
	config  config
	logger  *slog.Logger
	models  data.Models
	limiter *LimiterStore
	audit   *logrus.Logger
	client  *gocloak.GoCloak
}

func registerMetrics() {
	// Register Prometheus metrics only once
	reg := prometheus.DefaultRegisterer

	metrics := []prometheus.Collector{
		HttpRequestsTotal,
		HttpRequestDuration,
		HttpRequestSize,
		HttpResponseSize,
		HttpRequestsErrorsTotal,
		DbQueryErrorsTotal,
		PanicRecoveryTotal,
		DbQueryDuration,
		UserRegistrationsTotal,
		LoginsTotal,
		FailedLoginsTotal,
	}

	for _, metric := range metrics {
		if err := reg.Register(metric); err != nil {
			log.Println("Metric already registered, skipping:", err)
		}
	}
}

func startMonitoring(ctx context.Context, db *gorm.DB) {
	go func() {
		sqlDB, _ := db.DB()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var memStats runtime.MemStats
				runtime.ReadMemStats(&memStats)

				// Ensure metrics exist before updating them
				if prometheus.DefaultGatherer != nil {
					GoGoroutines.Set(float64(runtime.NumGoroutine()))
					GoMemAllocBytes.Set(float64(memStats.Alloc))
					GoMemHeapObjects.Set(float64(memStats.HeapObjects))
				}

				// Update active DB connections
				if prometheus.DefaultGatherer != nil {
					DbConnectionsActive.WithLabelValues("postgres").Set(float64(sqlDB.Stats().OpenConnections))
				}
			}
		}
	}()
}

func main() {
	var cfg config
	var rps float64
	var burst int

	flag.IntVar(&cfg.port, "port", 20000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("GREENLIGHT_DB_DSN"), "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")
	flag.Float64Var(&rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.StringVar(&cfg.kc.Realm, "realm", os.Getenv("KEYCLOAK_REALM"), "Keycloak Realm")
	flag.StringVar(&cfg.kc.AuthURL, "auth-url", os.Getenv("KEYCLOAK_AUTHURL"), "Keycloak Auth URL")
	flag.StringVar(&cfg.kc.admin_username, "admin-username", os.Getenv("KEYCLOAK_ADMIN"), "Keycloak Admin Username")
	flag.StringVar(&cfg.kc.admin_password, "admin-password", os.Getenv("KEYCLOAK_ADMIN_PASSWORD"), "Keycloak Admin Password")
	flag.StringVar(&cfg.kc.client_id, "client-id", os.Getenv("KEYCLOAK_CLIENT_ID"), "Keycloak Client ID")
	flag.StringVar(&cfg.kc.client_secret, "client-secret", os.Getenv("KEYCLOAK_CLIENT_SECRET"), "Keycloak Client Secret")
	flag.StringVar(&cfg.kc.kc_jwks_url, "jwks-url", os.Getenv("KEYCLOAK_JWKS_URL"), "Keycloak JWKS URL")
	flag.StringVar(&cfg.kc.kc_issuer_url, "issuer-url", os.Getenv("KEYCLOAK_ISSUER_URL"), "Keycloak Issuer URL")
	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		if val == "" {
			cfg.cors.trustedOrigins = []string{"http://example.com", "https://example2.com"}
			return nil
		}
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})

	flag.Parse()

	// Graceful shutdown context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Register Prometheus metrics
	registerMetrics()

	ltr := NewLimiterStore(rate.Limit(rps), burst)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	go UpdateMovieCount(db)
	logger.Info("database connection pool established")

	// Start monitoring goroutine with graceful shutdown support
	startMonitoring(ctx, db)

	auditLogger := logrus.New()
	client := gocloak.NewClient(cfg.kc.AuthURL)

	app := &application{
		config:  cfg,
		logger:  logger,
		models:  data.NewModels(db),
		limiter: ltr,
		audit:   auditLogger,
		client:  client,
	}

	// Handle shutdown signals
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		<-sigChan
		logger.Info("Shutting down gracefully...")
		cancel() // Stop goroutines
		os.Exit(0)
	}()

	router := app.routes()

	if err := router.Run(":" + strconv.Itoa(app.config.port)); err != nil {
		logger.Error("Cannot start the gin Router")
	}
}
