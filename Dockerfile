# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod ./

# Download dependencies (none for now, but good practice)
RUN go mod download

# Copy source code
COPY *.go ./
COPY frontend/ ./frontend/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o dashboard .

# Production stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/dashboard .

# Expose port
EXPOSE 43565

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:43565/health || exit 1

# Run
CMD ["./dashboard"]
