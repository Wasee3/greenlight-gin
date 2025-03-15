package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/gorm"

	"github.com/Wasee3/greenlight-gin/internal/data"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"gorm.io/driver/postgres"

	//Dynamic Service Discovery
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/hashicorp/consul/api"
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

func getContainerIP(containerName string) (string, error) {
	// fmt.Println(containerName)
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	defer cli.Close()
	// containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{All: true})
	containers, err := cli.ContainerList(context.Background(), container.ListOptions{All: true})
	if err != nil {
		return "", err
	}
	for _, container := range containers {
		// fmt.Println(container.Names)
		for _, name := range container.Names {
			// fmt.Println(name)
			if name == containerName { // Match container name
				containerJSON, err := cli.ContainerInspect(context.Background(), container.ID)
				// fmt.Println(containerJSON)
				if err != nil {
					return "", err
				}

				for _, network := range containerJSON.NetworkSettings.Networks {
					// fmt.Println(network.IPAddress)
					return network.IPAddress, nil // Return the first network IP found
				}
			}
		}
	}

	return "", fmt.Errorf("container %s not found", containerName)
}

// Function to query Consul for the Jaeger service
func getService(tagFilter string, targetPort int, consulIP string) (string, error) {
	consulAddr := fmt.Sprintf("%s:8500", consulIP)
	serviceName := fmt.Sprintf("%s-%d", tagFilter, targetPort)

	// Create a new Consul client
	config := api.DefaultConfig()
	config.Address = consulAddr
	client, err := api.NewClient(config)
	if err != nil {
		return "", err
	}

	instances, _, err := client.Health().Service(serviceName, tagFilter, true, nil)
	if err != nil {
		return "", errors.New("Problem found while quering service for instances")
	}

	if len(instances) == 0 {
		return "", errors.New("No healthy Service instances found")
	}

	// Pick a random instance
	selected := instances[rand.Intn(len(instances))]

	// Print the selected instance details
	fmt.Printf("Randomly selected Healthy instance: %s:%d\n",
		selected.Service.Address, selected.Service.Port)

	// Return the selected instance address
	return fmt.Sprintf("%s:%d", selected.Service.Address, targetPort), nil
}

func registerMetrics() {
	// Register Prometheus metrics only once
	reg := prometheus.DefaultRegisterer

	metrics := []prometheus.Collector{
		HttpRequestsTotal,
		HttpRequestDuration,
		HttpRequestSize,
		HttpResponseSize,
		HttpRequestsErrorsTotal,
		DbQueryErrorsTotal,
		PanicRecoveryTotal,
		DbQueryDuration,
		UserRegistrationsTotal,
		LoginsTotal,
		FailedLoginsTotal,
	}

	for _, metric := range metrics {
		if err := reg.Register(metric); err != nil {
			log.Println("Metric already registered, skipping:", err)
		}
	}
}

func startMonitoring(ctx context.Context, db *gorm.DB) {
	go func() {
		sqlDB, _ := db.DB()
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var memStats runtime.MemStats
				runtime.ReadMemStats(&memStats)

				// Ensure metrics exist before updating them
				if prometheus.DefaultGatherer != nil {
					GoGoroutines.Set(float64(runtime.NumGoroutine()))
					GoMemAllocBytes.Set(float64(memStats.Alloc))
					GoMemHeapObjects.Set(float64(memStats.HeapObjects))
				}

				// Update active DB connections
				if prometheus.DefaultGatherer != nil {
					DbConnectionsActive.WithLabelValues("postgres").Set(float64(sqlDB.Stats().OpenConnections))
				}
			}
		}
	}()
}

func initTracer(ctx context.Context) (*trace.TracerProvider, error) {
	consulIP, err := getContainerIP("/consul")
	// fmt.Println(consulIP)
	if consulIP == "" {
		return nil, fmt.Errorf("failed to get Consul container IP")
	}
	if err != nil {
		return nil, err
	}

	jaeger, err := getService("jaeger", 4317, consulIP)
	if err != nil {
		// fmt.Println(err)
		return nil, err
	}

	// // Set a timeout context for connection
	// cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()

	// Create a gRPC connection with a timeout using DialContext
	conn, err := grpc.NewClient(jaeger,
		grpc.WithTransportCredentials(insecure.NewCredentials()), // Use TLS in production
	)
	if err != nil {
		return nil, err
	}

	// Create an OTLP gRPC client
	client := otlptracegrpc.NewClient(
		otlptracegrpc.WithGRPCConn(conn),
		otlptracegrpc.WithEndpoint(jaeger),
	)

	// Create the OTLP Trace Exporter
	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		return nil, err
	}

	// Define OpenTelemetry resource attributes
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("greenlight-api"),
			semconv.ServiceVersionKey.String(version),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create a Trace Provider
	tp := trace.NewTracerProvider(
		trace.WithSpanProcessor(trace.NewBatchSpanProcessor(exporter)),
		trace.WithResource(res),
	)

	// Set the global TracerProvider
	otel.SetTracerProvider(tp)

	return tp, nil
}

func loadEnv(cfg *config) {

	env_vars := []string{"REQ_PER_SECOND", "BURST", "API_PORT", "GREENLIGHT_DB_DSN", "KEYCLOAK_REALM", "KEYCLOAK_AUTHURL", "KEYCLOAK_ADMIN", "KEYCLOAK_ADMIN_PASSWORD", "KEYCLOAK_CLIENT_ID", "KEYCLOAK_CLIENT_SECRET", "KEYCLOAK_JWKS_URL", "KEYCLOAK_ISSUER_URL"}

	for _, env_var := range env_vars {
		if os.Getenv(env_var) == "" {
			fmt.Println(env_var + " = " + os.Getenv(env_var))
			fmt.Println("Please set the below mentioned variables")
			fmt.Println("REQ_PER_SECOND  should be in Decimal format")
			fmt.Println("BURST should be in Integer format")
			fmt.Println("API_PORT should be in Integer format")
			fmt.Println("GREENLIGHT_DB_DSN=postgres://username:secret@localhost/dbname?sslmode=disable")
			fmt.Println("KEYCLOAK_REALM=greenlight (sample Realm from Keycloak)")
			fmt.Println("KEYCLOAK_AUTHURL=http://localhost:8080/auth/realms/greenlight/protocol/openid-connect/token")
			fmt.Println("KEYCLOAK_ADMIN=greenlight-admin")
			fmt.Println("KEYCLOAK_ADMIN_PASSWORD=secret")
			fmt.Println("KEYCLOAK_CLIENT_ID=greenlight-api")
			fmt.Println("KEYCLOAK_CLIENT_SECRET=secret")
			fmt.Println("KEYCLOAK_JWKS_URL=http://localhost:8080/realms/greenlight/protocol/openid-connect/certs")
			fmt.Println("KEYCLOAK_ISSUER_URL=http://localhost:8080/realms/greenlight")

			logrus.Fatalf("Environment variable %s not set", env_var)
		}
	}

	var rps float64
	var burst int

	rps, err := strconv.ParseFloat(os.Getenv("REQ_PER_SECOND"), 64)
	if err != nil {
		logrus.Fatal("Invalid value for REQ_PER_SECOND environment variable: ", err)
	}

	burst, err = strconv.Atoi(os.Getenv("BURST"))
	if err != nil {
		logrus.Fatal("Invalid value for BURST environment variable: ", err)
	}

	flag.StringVar(&cfg.port, "port", os.Getenv("API_PORT"), "API server port")
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)")
	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("GREENLIGHT_DB_DSN"), "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.DurationVar(&cfg.db.maxIdleTime, "db-max-idle-time", 15*time.Minute, "PostgreSQL max connection idle time")

	flag.Float64Var(&cfg.ltr_rps, "limiter-rps", rps, "Rate limiter maximum requests per second")

	flag.IntVar(&cfg.ltr_burst, "limiter-burst", burst, "Rate limiter maximum burst")
	flag.StringVar(&cfg.kc.Realm, "realm", os.Getenv("KEYCLOAK_REALM"), "Keycloak Realm")
	flag.StringVar(&cfg.kc.AuthURL, "auth-url", os.Getenv("KEYCLOAK_AUTHURL"), "Keycloak Auth URL")
	flag.StringVar(&cfg.kc.admin_username, "admin-username", os.Getenv("KEYCLOAK_ADMIN"), "Keycloak Admin Username")
	flag.StringVar(&cfg.kc.admin_password, "admin-password", os.Getenv("KEYCLOAK_ADMIN_PASSWORD"), "Keycloak Admin Password")
	flag.StringVar(&cfg.kc.client_id, "client-id", os.Getenv("KEYCLOAK_CLIENT_ID"), "Keycloak Client ID")
	flag.StringVar(&cfg.kc.client_secret, "client-secret", os.Getenv("KEYCLOAK_CLIENT_SECRET"), "Keycloak Client Secret")
	flag.StringVar(&cfg.kc.kc_jwks_url, "jwks-url", os.Getenv("KEYCLOAK_JWKS_URL"), "Keycloak JWKS URL")
	flag.StringVar(&cfg.kc.kc_issuer_url, "issuer-url", os.Getenv("KEYCLOAK_ISSUER_URL"), "Keycloak Issuer URL")
	flag.Func("cors-trusted-origins", "Trusted CORS origins (space separated)", func(val string) error {
		if val == "" {
			cfg.cors.trustedOrigins = []string{"http://example.com", "https://example2.com"}
			return nil
		}
		cfg.cors.trustedOrigins = strings.Fields(val)
		return nil
	})

	flag.Parse()

}
