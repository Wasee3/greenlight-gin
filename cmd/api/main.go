package main

import (
	"context"
	"log/slog"
	"os"
	"syscall"
	"time"

	"os/signal"

	"github.com/Nerzal/gocloak/v13"
	"github.com/Wasee3/greenlight-gin/internal/data"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	// OpenTelemetry imports

	oteltrace "go.opentelemetry.io/otel/trace"
)

const version = "1.0.0"

type config struct {
	port      string
	env       string
	ltr_rps   float64
	ltr_burst int
	db        struct {
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
	tracer  oteltrace.Tracer
}

func main() {

	var cfg config
	loadEnv(&cfg)
	// Graceful shutdown context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// **Initialize OpenTelemetry**
	tp, err := initTracer(ctx)
	if err != nil {
		logrus.Fatal("Failed to initialize OpenTelemetry: ", err)
	}
	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			logrus.Fatal("Error shutting down tracer provider: ", err)
		}
	}()

	// Register Prometheus metrics
	registerMetrics()

	ltr := NewLimiterStore(rate.Limit(cfg.ltr_rps), cfg.ltr_burst)

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
		tracer:  tp.Tracer("greenlight-api"),
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

	if err := router.Run(":" + app.config.port); err != nil {
		logger.Error("Cannot start the gin Router")
	}
}
