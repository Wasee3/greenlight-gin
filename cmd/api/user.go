package main

type User struct {
	Username  string `json:"username" binding:"required,min=2,max=20"`
	Email     string `json:"email" binding:"required,email,max=40"`
	Password  string `json:"password" binding:"required,min=10,max=20"`
	FirstName string `json:"first_name" binding:"required,min=2,max=20"`
	LastName  string `json:"last_name" binding:"required,min=2,max=20"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type PasswordChangeRequest struct {
	Username string `json:"username" binding:"required"`
}
