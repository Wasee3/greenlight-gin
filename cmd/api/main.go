package main

import (
	"flag"
	"log/slog"
	"os"

	"github.com/gin-gonic/gin"
)

const version = "1.0.0"

type config struct {
	port int
	env  string
}

type application struct {
	config config
	logger *slog.Logger
}

func main() {

	var cfg config

	flag.IntVar(&cfg.port, "port", 4000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	app := &application{
		config: cfg,
		logger: logger,
	}

	router := gin.Default()
	router.GET("/v1/healthcheck", app.healthcheckHandler)

	// srv := &http.Server{
	// 	Addr:         fmt.Sprintf(":%d", cfg.port),
	// 	Handler:      mux,
	// 	IdleTimeout:  time.Minute,
	// 	ReadTimeout:  5 * time.Second,
	// 	WriteTimeout: 10 * time.Second,
	// 	ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
	// }
	// Start the HTTP server.
	// logger.Info("starting server", "addr", srv.Addr, "env", cfg.env)
	err := router.Run()
	logger.Error(err.Error())
	os.Exit(1)
}
