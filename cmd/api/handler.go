package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Nerzal/gocloak/v13"
	"github.com/Wasee3/greenlight-gin/internal/data"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/copier"
	"gorm.io/gorm"
)

func (app *application) ShowMovieHandler(c *gin.Context) {
	idStr := c.Params.ByName("id")

	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing id parameter"})
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id parameter"})
		return
	}

	var movie *data.Movies
	movie, err = app.models.Movies.Get(c, id)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) { // Handle "not found" error specifically
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Movie with ID %d not found", id)})
		} else {
			app.logger.Error("Database error", "error", err) // Log unexpected errors
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		}
		return
	}

	var input data.Input
	err = copier.Copy(&input, &movie)
	if err != nil {
		app.logger.Error("Copier error", "error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error by Copier"})
		return
	}
	c.JSON(http.StatusOK, input)

}

func (app *application) CreateMovieHandler(c *gin.Context) {

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1048576)

	var input data.Input

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	movie := &data.Movies{
		ID:        input.ID,
		CreatedAt: time.Now(),
		Title:     input.Title,
		Year:      input.Year,
		Runtime:   input.Runtime,
		Genres:    input.Genres,
		Version:   1,
	}

	err := app.models.Movies.Insert(c, movie)

	if err != nil {
		app.logger.Error("Failed to insert movie", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Data received successfully",
		"data":    input,
	})
}

func (app *application) UpdateMovieHandler(c *gin.Context) {
	// Limit request body size to 1MB
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1048576)

	idStr := c.Param("id")

	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing id parameter"})
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id parameter"})
		return
	}

	// Bind JSON request body to `update` struct
	var update data.Update
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Update the movie inside a transaction
	updatedMovie, err := app.models.Movies.UpdateMovieInTransaction(c, id, update)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Movie with ID %d not found", id)})
		} else if strings.HasPrefix(err.Error(), "concurrent_update:") {
			c.JSON(http.StatusConflict, gin.H{"error": "Movie was modified by another request. Please retry."})
		} else {
			app.logger.Error("Failed to update movie", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		}
		return
	}

	// Respond with the updated movie data
	c.JSON(http.StatusOK, gin.H{"message": "Movie updated successfully", "movie": updatedMovie})
}

func (app *application) DeleteMovieHandler(c *gin.Context) {
	idStr := c.Params.ByName("id")

	if idStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing id parameter"})
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid id parameter"})
		return
	}

	err = app.models.Movies.Delete(c, id)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("Movie with ID %d not found", id)})
		} else {
			app.logger.Error("Database error", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"Message": fmt.Sprintf("Movie with ID %d deleted", id)})
}

func (app *application) ListMovieHandler(c *gin.Context) {
	filter := &data.Filters{
		Page:     1,
		PageSize: 2,
		Sort:     "id",
		Order:    "asc",
		Title:    "",
		Pretty:   false,
	}

	if err := c.ShouldBindQuery(&filter); err != nil {
		app.logger.Error("Invalid Query", "Error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var movies *[]data.Movies
	var err error
	var tr int64
	var metadata *data.Metadata

	if filter.Title != "" {
		filter.Pretty = true
		movies, tr, err = app.models.Movies.Search(c, filter)
		metadata = &data.Metadata{
			CurrentPage:  filter.Page,
			PageSize:     filter.PageSize,
			FirstPage:    1,
			LastPage:     int(math.Ceil(float64(tr) / float64(filter.PageSize))),
			TotalRecords: int64(tr),
		}
	} else {
		movies, err = app.models.Movies.List(c, filter)
		metadata = &data.Metadata{
			CurrentPage:  filter.Page,
			PageSize:     filter.PageSize,
			FirstPage:    1,
			LastPage:     int(math.Ceil(float64(tr) / float64(filter.PageSize))),
			TotalRecords: int64(totalMoviesCount),
		}
	}

	if err != nil {
		app.logger.Error("Unable to list movie", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var input []data.Input
	err = copier.Copy(&input, &movies)
	if err != nil {
		app.logger.Error("Copier error", "error:", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if filter.Pretty {
		c.IndentedJSON(http.StatusOK, gin.H{"Metadata": metadata, "movies": input})
	} else {
		c.JSON(http.StatusOK, gin.H{"Metadata": metadata, "movies": input})
	}
}

func (app *application) RegisterUserHandler(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1048576)

	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		app.logger.Error("Invalid Input", "error", err)
		c.JSON(http.StatusBadRequest,
			gin.H{
				"required_fields": gin.H{
					"Username":   "Allowed chars: a-z, A-Z, 0-9, _ pr - min: 2, max: 20",
					"Email":      "Valid Email, max: 40",
					"Password":   "Allowed chars: a-z, A-Z, 0-9, _, -, @ min: 10, max: 20",
					"First Name": "Allowed chars: a-z, A-Z, - min: 2, max: 20",
					"Last Name":  "Allowed chars: a-z, A-Z, - min: 2, max: 20",
				},
				"error": err.Error(),
			})
		return
	}

	match, err := regexp.MatchString(`^[a-zA-Z0-9_]+$`, user.Username)
	if !match {
		app.logger.Error("Invalid Username", "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Username"})
		return
	}

	match, err = regexp.MatchString(`^[a-zA-Z0-9_@]+$`, user.Password)
	if !match {
		app.logger.Error("Invalid Password", "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Password"})
		return
	}

	match, err = regexp.MatchString(`^[a-zA-Z-]+$`, user.FirstName)
	if !match {
		app.logger.Error("Invalid First Name", "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid First Name"})
		return
	}

	match, err = regexp.MatchString(`^[a-zA-Z-]+$`, user.LastName)
	if !match {
		app.logger.Error("Invalid Last Name", "error", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Last Name"})
		return
	}

	keycloakURL := app.config.kc.AuthURL
	realm := app.config.kc.Realm
	admin_username := app.config.kc.admin_username
	admin_password := app.config.kc.admin_password

	client := gocloak.NewClient(keycloakURL)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 180*time.Second)
	defer cancel()

	token, err := client.LoginAdmin(ctx, admin_username, admin_password, realm)
	if err != nil {
		app.logger.Error("Failed to login to Keycloak", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	users, err := client.GetUsers(ctx, token.AccessToken, realm, gocloak.GetUsersParams{
		Username: &user.Username,
		Max:      gocloak.IntP(1), // Get only 1 user
	})
	if err != nil {
		app.logger.Error("Failed to search user in Keycloak", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if len(users) > 0 {
		app.logger.Warn("User Exists, Cannot create", "username", user.Username)
		c.JSON(http.StatusBadRequest, gin.H{"error": "User already exists", "username": user.Username})
		return
	}

	kcuser := gocloak.User{
		Username:      gocloak.StringP(user.Username),
		FirstName:     gocloak.StringP(user.FirstName),
		LastName:      gocloak.StringP(user.LastName),
		Email:         gocloak.StringP(user.Email),
		Enabled:       gocloak.BoolP(true),
		EmailVerified: gocloak.BoolP(true),
		Credentials: &[]gocloak.CredentialRepresentation{
			{
				Type:      gocloak.StringP("password"),
				Value:     gocloak.StringP(user.Password),
				Temporary: gocloak.BoolP(false),
			},
		},
	}

	_, err = client.CreateUser(ctx, token.AccessToken, realm, kcuser)
	if err != nil {
		app.logger.Error("Failed to create user in Keycloak", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"message": "Data received successfully", "data": user})

}

func (app *application) LoginUserHandler(c *gin.Context) {
	var req LoginRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		app.logger.Error("Invalid Input", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	client := gocloak.NewClient(app.config.kc.AuthURL)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 180*time.Second)
	defer cancel()

	token, err := client.Login(ctx, app.config.kc.client_id, app.config.kc.client_secret, app.config.kc.Realm, req.Username, req.Password)

	if err != nil {
		app.logger.Error("Failed to login to Keycloak", "error", err, "username", req.Username, "realm", app.config.kc.Realm, "client_id", app.config.kc.client_id)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{"access_token": token.AccessToken, "refresh_token": token.RefreshToken, "expires_in": token.ExpiresIn, "token_type": token.TokenType})
}
