package main

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
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
