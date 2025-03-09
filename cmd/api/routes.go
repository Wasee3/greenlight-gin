package main

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (app *application) routes() *gin.Engine {

	router := gin.Default()

	router.Use(app.RateLimiterMiddleware())
	router.GET("/v1/healthcheck", app.healthcheckHandler)
	router.POST("/v1/user/register", app.RegisterUserHandler)
	router.POST("/v1/user/login", app.LoginUserHandler)
	router.POST("/v1/user/password/reset", app.PasswordResetHandler)

	router.GET("/v1/movie/:id", app.JWTAuthMiddleware([]string{"reader"}), app.ShowMovieHandler)
	router.POST("/v1/movie", app.JWTAuthMiddleware([]string{"writer"}), app.CreateMovieHandler)
	router.GET("/v1/movie", app.JWTAuthMiddleware([]string{"reader"}), app.ListMovieHandler)
	router.PUT("/v1/movie/:id", app.JWTAuthMiddleware([]string{"writer"}), app.UpdateMovieHandler)
	router.DELETE("/v1/movie/:id", app.JWTAuthMiddleware([]string{"writer"}), app.DeleteMovieHandler)
	router.POST("/v1/token/refresh", app.JWTAuthMiddleware([]string{"writer"}), app.RefreshTokenHandler)
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	return router
}
