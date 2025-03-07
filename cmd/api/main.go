package main

import (
	"flag"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/Wasee3/greenlight-gin/internal/data"
	"golang.org/x/time/rate"
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
}

type application struct {
	config  config
	logger  *slog.Logger
	models  data.Models
	limiter *LimiterStore
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
	// flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

	flag.Parse()

	ltr := NewLimiterStore(rate.Limit(rps), burst)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	db, err := openDB(cfg)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	go UpdateMovieCount(db)
	logger.Info("database connection pool established")

	app := &application{
		config:  cfg,
		logger:  logger,
		models:  data.NewModels(db),
		limiter: ltr,
	}

	router := app.routes()

	err = router.Run(":" + strconv.Itoa(app.config.port))

	if err != nil {
		logger.Error("Cannot start the gin Router")
	}

	os.Exit(0)
}
