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

# Set the working directory inside the container
WORKDIR /root/

RUN mkdir -p /etc/connector

# Copy the Go binary from the builder stage
COPY --from=builder /app/ndc-elasticsearch .

# Expose the port on which the service will run
EXPOSE 8080

ENV HASURA_CONFIGURATION_DIRECTORY=/etc/connector

# Run the web service on container startup.
ENTRYPOINT [ "./ndc-elasticsearch" ]
CMD [ "serve" ]