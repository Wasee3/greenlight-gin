# Greenlight Gin REST API

Recently, I read a book about writing APIs in Go using the `net/http` framework.

It was a wonderful book that helped me learn a lot. I implemented a REST API in Go at work, leveraging the knowledge gained.

Although that book was based on a different web library framework, I decided to adapt that framework to the **Gin Web Framework**.

I've tried to use a number of **production-grade microservice tools** just for illustration purposes.

## 🚀 Tools Used:
- **Gin** (Web Framework)
- **GORM** (ORM for interacting with Postgres DB)
- **Postgres** (Database)
- **Keycloak** (Identity Provider)
- **Consul** (Service Registry)
- **Registrator** (Dynamic Discovery)
- **slog** (Structured Logging)
- **Prometheus Client** (Metrics)
- **OpenTelemetry** (Emitting Traces)
- **Jaeger** (APM)

---

## ⚙️ Makefile Targets

Run `make` to see the available targets:

```bash
$ make
Available targets:
help : Show available targets
db-migrations-up : Apply database migrations
db-migrations-down : Rollback database migrations
audit : Tidy dependencies and format, vet, and test all code
vendor : Tidy and vendor dependencies
docker-network : Create the microservice network
docker-services : Start required Docker services
docker-clean : Stop and remove all services
docker-network-clean : Remove the microservice network
docker-ps : List running containers
docker-build : Build Docker image
docker-run-api : Run Docker container
docker-stop-api : Stop Docker container
```

Feel free to clone, edit and learn!!!!
