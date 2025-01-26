# Use a minimal base image with Go runtime
FROM docker.io/golang:1.23-alpine as builder

# Set environment variables
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

# Set the working directory inside the container
WORKDIR /app

# Copy Go module manifests and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o bin/app ./cmd

# Create a minimal runtime image
FROM alpine:3.18

# Set the working directory inside the container
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/bin/app .

# Expose the HTTP port for health checks
EXPOSE 8080

# Start the application
ENTRYPOINT ["./app"]
