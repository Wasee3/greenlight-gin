package main

import (
	"github.com/gin-gonic/gin"
)

func (app *application) healthcheckHandler(c *gin.Context) {

	c.JSON(200, gin.H{"status": "available",
		"env":     app.config.env,
		"version": version})
}
