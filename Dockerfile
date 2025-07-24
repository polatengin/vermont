FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN make build

# Final stage - using distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12

# Copy binary from builder
COPY --from=builder /app/bin/vermont /vermont

# Set the binary as entrypoint to accept dynamic commands
ENTRYPOINT ["/vermont"]
