package main

import (
	"github.com/gin-gonic/gin"
)

func (app *application) healthcheckHandler(c *gin.Context) {

	c.IndentedJSON(200, gin.H{
		"status": "available",
		"system_info": gin.H{
			"environment": app.config.env,
			"version":     version,
		},
	})
}
