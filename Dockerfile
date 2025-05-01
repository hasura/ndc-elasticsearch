# Stage 1: Build the Go binary
FROM golang:1.23 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build the Go application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ndc-elasticsearch

# Stage 2: Create a minimal image with the Go binary
FROM alpine:3

# Install necessary certificates for the application to run
RUN apk --no-cache add ca-certificates

# Create a safe working directory
WORKDIR /app

RUN mkdir -p /etc/connector

# Copy the Go binary from the builder stage
COPY --from=builder /app/ndc-elasticsearch .

# Create non-root user with UID and GID 1001
RUN addgroup -g 1001 hasura && \
    adduser -u 1001 -G hasura -D hasura && \
    chown 1001:1001 /app/ndc-elasticsearch && \
    chmod 755 /app/ndc-elasticsearch

# Use the non-root user
USER 1001

# Expose port
EXPOSE 8080

# Set env if needed
ENV HASURA_CONFIGURATION_DIRECTORY=/etc/connector

# Run the app
ENTRYPOINT ["/app/ndc-elasticsearch"]
CMD ["serve"]