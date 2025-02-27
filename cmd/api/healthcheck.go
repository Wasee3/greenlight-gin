package main

import (
	"github.com/gin-gonic/gin"
)

// Declare a handler which writes a plain-text response with information about the
// application status, operating environment and version.

// func (app *application) healthcheckHandler(w http.ResponseWriter, r *http.Request) {
// 	fmt.Fprintln(w, "status: available")
// 	fmt.Fprintf(w, "environment: %s\n", app.config.env)
// 	fmt.Fprintf(w, "version: %s\n", version)
// }

func (app *application) healthcheckHandler(c *gin.Context) {

	c.JSON(200, gin.H{"status": "available",
		"env":     app.config.env,
		"version": version})
}
