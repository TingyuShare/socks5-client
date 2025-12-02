# Stage 1: Build the application
# Use the official Go image that matches the version in go.mod
FROM golang:1.24.5-alpine AS builder

# Set the working directory
WORKDIR /app

# Copy go.mod and go.sum to leverage Docker's layer caching
COPY go.mod go.sum ./
# Download dependencies
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go application, creating a statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -o /app/smart-proxy .

# Stage 2: Create the final, lightweight production image
FROM alpine:latest

# Set the working directory
WORKDIR /app

# Copy the compiled binary from the 'builder' stage
COPY --from=builder /app/smart-proxy .

# Copy the GeoLite2 database file required by the application
COPY GeoLite2-Country.mmdb .

# Expose the default SOCKS5 port the application listens on
EXPOSE 1088

# Set the entrypoint for the container.
# The default command listens on 0.0.0.0:1088 inside the container.
ENTRYPOINT ["./smart-proxy"]

# Default arguments. Users can override these in `docker run`.
# Example: docker run <image> --proxy <your-upstream-proxy>
CMD ["--listen", "0.0.0.0:1088"]