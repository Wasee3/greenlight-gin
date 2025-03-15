# ------------------------------------------------------------
# 1) Build Stage
# ------------------------------------------------------------
FROM golang:1.24 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker layer caching
COPY go.mod go.sum ./

# Download and cache Go modules
#RUN go mod download

# Copy the rest of your application code
COPY . .

# Move into the cmd/api folder for building
WORKDIR /app/cmd/api

# Build the Go binary (static build)
RUN CGO_ENABLED=0 GOOS=linux go build -o /greenlight

# ------------------------------------------------------------
# 2) Final Stage
# ------------------------------------------------------------
FROM alpine:latest

# Copy the compiled binary from the builder stage
COPY --from=builder /greenlight /greenlight

# Expose the port your Go application listens on (default 4000)
EXPOSE 20000

# Define the container's startup command
ENTRYPOINT ["/greenlight"]

