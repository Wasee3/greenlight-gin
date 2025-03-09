package main

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/Wasee3/greenlight-gin/internal/data"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var (
	totalMoviesCount int64
	countMutex       sync.RWMutex
)

func openDB(cfg config) (*gorm.DB, error) {
	dsn := cfg.db.dsn
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// sqlDB, err := db.DB()
	// if err != nil {
	// 	log.Fatal("Failed to get database instance:", err)
	// }

	// Set connection pooling parameters
	// sqlDB.SetMaxOpenConns(app.cfg.maxOpenConns)                    // Maximum number of open connections
	// sqlDB.SetMaxIdleConns(app.cfg.maxIdleConns)                    // Maximum number of idle connections
	// sqlDB.SetConnMaxIdleTime(app.cfg.maxIdleTime)    // Idle connection timeout
	// app.logger.Info("database connection pool established")
	return db, nil
}

func UpdateMovieCount(db *gorm.DB) {
	ticker := time.NewTicker(30 * time.Second) // Adjust interval as needed
	defer ticker.Stop()

	for range ticker.C {
		var count int64
		_ = db.Model(&data.Movies{}).Count(&count).Error

		// Safely update the count
		countMutex.Lock()
		totalMoviesCount = count
		countMutex.Unlock()
	}
}

// NewLimiterStore initializes the store
func NewLimiterStore(r rate.Limit, b int) *LimiterStore {
	store := &LimiterStore{
		clients: make(map[string]*ClientLimiter),
		r:       r,
		b:       b,
	}

	// Start a cleanup routine to remove inactive clients
	go store.cleanupStaleClients()

	return store
}

func (s *LimiterStore) GetLimiter(ip string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If client exists, update LastSeen time
	if client, exists := s.clients[ip]; exists {
		client.LastSeen = time.Now()
		return client.Limiter
	}

	// Create a new limiter for a new IP
	limiter := rate.NewLimiter(s.r, s.b)
	s.clients[ip] = &ClientLimiter{
		Limiter:  limiter,
		LastSeen: time.Now(),
	}

	return limiter
}

func (s *LimiterStore) cleanupStaleClients() {
	for {
		time.Sleep(10 * time.Minute) // Run cleanup every 10 minutes
		s.mu.Lock()
		for ip, client := range s.clients {
			if time.Since(client.LastSeen) > 15*time.Minute { // Remove if inactive for 15 min
				delete(s.clients, ip)
			}
		}
		s.mu.Unlock()
	}
}

// Fetch JWKs from Keycloak
func (app *application) fetchJWKs(c *gin.Context) (jwk.Set, error) {
	keycloakJWKS := app.config.kc.kc_jwks_url
	ctx, cancel := context.WithTimeout(c, 100*time.Second)
	defer cancel()

	set, err := jwk.Fetch(ctx, keycloakJWKS)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch JWKS: %w", err)
	}
	return set, nil
}

func extractRoles(claims jwt.MapClaims) []string {
	var roles []string
	// if realmAccess, ok := claims["realm_access"].(map[string]interface{}); ok {
	if realmAccess, ok := claims["resource_access"].(map[string]any); ok {
		if goGinApi, ok := realmAccess["go-gin-api"].(map[string]any); ok {
			if roleList, ok := goGinApi["roles"].([]any); ok {
				for _, role := range roleList {
					roles = append(roles, role.(string))
				}
			}
		}
	}
	return roles
}

// Check if user has required role
func hasRequiredRole(userRoles, requiredRoles []string) bool {
	for _, reqRole := range requiredRoles {
		if slices.Contains(userRoles, reqRole) {
			return true
		}
	}
	return false
}

// Audit log function
func (app *application) auditLog(c *gin.Context, action, message string) {
	logEntry := logrus.Fields{
		"method":  c.Request.Method,
		"path":    c.Request.URL.Path,
		"ip":      c.ClientIP(),
		"action":  action,
		"message": message,
	}
	if user, exists := c.Get("user"); exists {
		logEntry["user"] = user
	}
	app.audit.WithFields(logEntry).Info()
}
