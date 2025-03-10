package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/time/rate"
)

type ClientLimiter struct {
	Limiter  *rate.Limiter
	LastSeen time.Time
}

// LimiterStore manages rate limiters per client
type LimiterStore struct {
	clients map[string]*ClientLimiter
	mu      sync.Mutex
	r       rate.Limit
	b       int
}

var (
	globalLimiter = rate.NewLimiter(20, 50) // Global: 20 req/sec, burst 50
	// clientStore   = NewLimiterStore(5, 10)  // Per-Client: 5 req/sec, burst 10
	mu sync.Mutex
)

func (app *application) RateLimiterMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		// Global rate limiting
		mu.Lock()
		if !globalLimiter.Allow() {
			mu.Unlock()
			app.logger.Error("Global Middleware ", "error:", errors.New("too many requests"))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Global rate limit exceeded"})
			c.Abort()
			return
		}
		mu.Unlock()

		// Per-client rate limiting
		limiter := app.limiter.GetLimiter(ip)
		if !limiter.Allow() {
			app.logger.Error("Client IP Middleware ", "error:", errors.New("too many requests"))
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests from your IP"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// Middleware: Validate JWT and extract roles
func (app *application) JWTAuthMiddleware(requiredRoles []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			app.auditLog(c, "UNAUTHORIZED", "Missing Authorization header")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Missing Authorization header"})
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		keySet, err := app.fetchJWKs(c)
		if err != nil {
			app.auditLog(c, "ERROR", "Failed to fetch Keycloak JWKS")
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
			return
		}

		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
			key, _ := keySet.Get(0)
			var rawKey any
			if err := key.Raw(&rawKey); err != nil {
				return nil, fmt.Errorf("failed to get raw key: %w", err)
			}
			return rawKey, nil
		})
		if err != nil || !token.Valid {
			app.auditLog(c, "UNAUTHORIZED", "Invalid or expired token")
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		// Extract roles from token
		claims, _ := token.Claims.(jwt.MapClaims)
		realmRoles := extractRoles(claims)

		// Enforce RBAC
		if !hasRequiredRole(realmRoles, requiredRoles) {
			app.auditLog(c, "FORBIDDEN", "User lacks required role")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Access Denied"})
			return
		}

		// Attach user info to context
		c.Set("user", claims["preferred_username"])
		app.auditLog(c, "ACCESS_GRANTED", "User authorized")
		c.Next()
	}
}

// CORSMiddleware handles CORS and preflight requests
func (app *application) CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origins := strings.Join(app.config.cors.trustedOrigins, ", ")
		c.Writer.Header().Set("Access-Control-Allow-Origin", origins) // Allow all origins, change to specific domain in production
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true") // Allow credentials (cookies, authorization headers)

		// Handle Preflight (OPTIONS request)
		if c.Request.Method == "OPTIONS" {
			c.Writer.WriteHeader(http.StatusNoContent) // 204 No Content response
			c.Abort()
			return
		}

		c.Next()
	}
}

func (app *application) PrometheusMiddleware(c *gin.Context) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Process request
		c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		statusCode := c.Writer.Status()

		HttpRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), strconv.Itoa(statusCode)).Inc()
		HttpRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(duration)
		HttpRequestSize.WithLabelValues(c.Request.Method, c.FullPath()).Observe(float64(c.Request.ContentLength))
		HttpResponseSize.WithLabelValues(c.Request.Method, c.FullPath()).Observe(float64(c.Writer.Size()))
		if statusCode >= 400 {
			HttpRequestsErrorsTotal.WithLabelValues(c.Request.Method, c.FullPath(), strconv.Itoa(statusCode)).Inc()
		}

	}
}

func (app *application) TraceMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		tracer := app.tracer
		spanCtx, span := tracer.Start(ctx, c.Request.Method+" "+c.FullPath())
		defer span.End()

		// Extract and add Trace ID for logging
		traceID := span.SpanContext().TraceID().String()
		c.Set("traceID", traceID)

		// Pass context with span to next handler
		c.Request = c.Request.WithContext(spanCtx)

		// Continue processing request
		c.Next()

		// Capture request metadata using custom attributes
		span.SetAttributes(
			attribute.String("http.method", c.Request.Method),          // HTTP method (e.g., GET, POST)
			attribute.String("http.target", c.Request.URL.Path),        // Request path
			attribute.String("http.user_agent", c.Request.UserAgent()), // User-Agent header
			attribute.String("http.client_ip", c.ClientIP()),           // Client IP address
			attribute.Int("http.status_code", c.Writer.Status()),       // HTTP status code
		)
	}
}
