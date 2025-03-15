.DEFAULT_GOAL := help

include .envrc

DOCKER_IMAGE ?= greenlight-gin
DOCKER_NETWORK ?= microservice_net

.PHONY: help # Show available targets
help:
	@echo "Available targets:"
	@sed -n 's/^\.PHONY: //p' $(MAKEFILE_LIST) | sed -E 's/\s+#\s+/ : /'


.PHONY: db-migrations-up # Apply database migrations
db-migrations-up:
	@echo 'Running up migrations...'
	migrate -path="./migrations" -database "${GREENLIGHT_DB_DSN_MIG}" up

.PHONY: db-migrations-down # Rollback database migrations
db-migrations-down:
	@echo 'Running down migrations...'
	migrate -path="./migrations" -database "${GREENLIGHT_DB_DSN_MIG}" down

.PHONY: audit # Tidy dependencies and format, vet, and test all code
audit: vendor
	@echo 'Formatting code...'
	go fmt ./...
	@echo 'Vetting code...'
	go vet ./...
	staticcheck ./...
	@echo 'Running tests...'
	go test -race -vet=off ./...

.PHONY: vendor # Tidy and vendor dependencies
vendor:
	@echo 'Tidying and verifying module dependencies'
	go mod tidy
	go mod verify
	@echo 'Vendoring dependencies'
	go mod vendor

.PHONY: docker-network # Create the microservice network
docker-network:
	@echo "Creating Docker network $(DOCKER_NETWORK)..."
	@docker network inspect $(DOCKER_NETWORK) >/dev/null 2>&1 || \
	docker network create --driver bridge --subnet=172.30.0.0/24 $(DOCKER_NETWORK)

.PHONY: docker-services # Start required Docker services
docker-services: docker-network
	@echo "Starting Consul..."
	docker run -d --name=consul --network=$(DOCKER_NETWORK) -p 8500:8500 \
		hashicorp/consul agent -server -bootstrap -client=0.0.0.0 -ui
	
	@echo "Starting Registrator..."
	docker run -d --name=registrator --network=$(DOCKER_NETWORK) \
		-v /var/run/docker.sock:/tmp/docker.sock gliderlabs/registrator \
		-internal -cleanup -resync 10 consul://consul:8500

	@echo "Starting OpenTelemetry Collector..."
	docker run -d -p 4317:4317 -p 4318:4318 -p 55679:55679 -p 55680:55680 \
		--network=$(DOCKER_NETWORK) \
		-v ./otel-config.yaml:/etc/otelcol/config.yaml \
		--name otel-collector \
		--label "SERVICE_TAGS=otel-collector" \
		--label "SERVICE_NAME=otel-collector" \
		otel/opentelemetry-collector-contrib:latest

.PHONY: docker-clean # Stop and remove all services
docker-clean:
	@echo "Stopping all running services..."
	docker stop consul registrator otel-collector jaeger keycloak || true
	@echo "Removing containers..."
	docker rm consul registrator otel-collector jaeger keycloak || true

.PHONY: docker-network-clean # Remove the microservice network
docker-network-clean:
	@echo "Removing Docker network $(DOCKER_NETWORK)..."
	docker network rm $(DOCKER_NETWORK) || true

.PHONY: docker-ps # List running containers
docker-ps:
	@echo "Listing running Docker containers..."
	docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Status}}"

.PHONY: docker-build # Build Docker image
docker-build:
	@echo 'Vendoring dependencies'
	go mod vendor
	@echo "Building Docker image $(DOCKER_IMAGE)..."
	docker build -t $(DOCKER_IMAGE) .
	

.PHONY: docker-run-api # Run Docker container
docker-run-api:
	@echo "Running Docker container $(DOCKER_IMAGE)..."
	docker run -d --name greenlight -p 20000:20000 --network=$(DOCKER_NETWORK) \
		-e GREENLIGHT_DB_DSN=${GREENLIGHT_DB_DSN} \
		-e KEYCLOAK_ADMIN=${KEYCLOAK_ADMIN} \
		-e KEYCLOAK_ADMIN_PASSWORD=${KEYCLOAK_ADMIN_PASSWORD} \
		-e KEYCLOAK_AUTHURL=${KEYCLOAK_AUTHURL} \
		-e KEYCLOAK_REALM=${KEYCLOAK_REALM} \
		-e KEYCLOAK_CLIENT_ID=${KEYCLOAK_CLIENT_ID} \
		-e KEYCLOAK_CLIENT_SECRET=${KEYCLOAK_CLIENT_SECRET} \
		-e KEYCLOAK_ISSUER_URL=${KEYCLOAK_ISSUER_URL} \
		-e KEYCLOAK_JWKS_URL=${KEYCLOAK_JWKS_URL} \
		-e REQ_PER_SECOND=${REQ_PER_SECOND} \
		-e BURST=${BURST} \
		-e API_PORT=${API_PORT} \
		-v /var/run/docker.sock:/var/run/docker.sock \
		--label "SERVICE_NAME=greenlight" \
		--label "SERVICE_TAGS=greenlight" \
		greenlight-gin:latest

.PHONY: docker-stop-api # Stop Docker container
docker-stop-api:
	@echo "Stopping Docker container $(DOCKER_IMAGE)..."
	docker stop greenlight || true
	docker rm greenlight || true
