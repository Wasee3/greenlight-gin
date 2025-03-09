package main

import (
	"github.com/gin-gonic/gin"
)

func (app *application) routes() *gin.Engine {

	router := gin.Default()

	// router.Use(gin.Recovery(), app.RateLimiterMiddleware())
	router.GET("/v1/healthcheck", app.RateLimiterMiddleware(), app.healthcheckHandler)
	router.POST("/v1/user/register", app.RateLimiterMiddleware(), app.RegisterUserHandler)
	router.POST("/v1/user/login", app.RateLimiterMiddleware(), app.LoginUserHandler)
	router.POST("/v1/user/password/reset", app.RateLimiterMiddleware(), app.PasswordResetHandler)

	// router.Use(gin.Recovery(), app.RateLimiterMiddleware())
	router.GET("/v1/movie/:id", app.RateLimiterMiddleware(), app.JWTAuthMiddleware([]string{"reader"}), app.ShowMovieHandler)
	router.POST("/v1/movie", app.RateLimiterMiddleware(), app.JWTAuthMiddleware([]string{"writer"}), app.CreateMovieHandler)
	router.GET("/v1/movie", app.RateLimiterMiddleware(), app.JWTAuthMiddleware([]string{"reader"}), app.ListMovieHandler)
	router.PUT("/v1/movie/:id", app.RateLimiterMiddleware(), app.JWTAuthMiddleware([]string{"writer"}), app.UpdateMovieHandler)
	router.DELETE("/v1/movie/:id", app.RateLimiterMiddleware(), app.JWTAuthMiddleware([]string{"writer"}), app.DeleteMovieHandler)
	router.POST("/v1/token/refresh", app.RateLimiterMiddleware(), app.JWTAuthMiddleware([]string{"writer"}), app.RefreshTokenHandler)

	return router
}
