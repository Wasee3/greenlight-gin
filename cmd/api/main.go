package main

import (
	"flag"
	"log/slog"
	"os"
	"strconv"

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

	flag.IntVar(&cfg.port, "port", 20000, "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	app := &application{
		config: cfg,
		logger: logger,
	}

	router := gin.Default()

	router.Use(gin.Recovery())
	router.GET("/v1/healthcheck", app.healthcheckHandler)
	router.GET("/v1/movie/:id", app.showMovieHandler)
	router.POST("/v1/movie", app.createMovieHandler)

	err := router.Run(":" + strconv.Itoa(app.config.port))
	logger.Error(err.Error())
	os.Exit(1)
}
