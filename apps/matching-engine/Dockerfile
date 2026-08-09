# Build stage
FROM golang:1.25.0-alpine AS builder

WORKDIR /app

# Copy the monorepo content
COPY . .

# Build the binary
WORKDIR /app/apps/matching-engine
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main .

# Run stage
FROM alpine:3.19.1

# Create non-privileged user and group
RUN addgroup -S velyxgroup && adduser -S velyxuser -G velyxgroup

WORKDIR /app

COPY --from=builder /app/apps/matching-engine/main .

# Enforce secure ownership and permissions
RUN chown -R velyxuser:velyxgroup /app

# Run as non-root user
USER velyxuser

# Clean signal propagation
STOPSIGNAL SIGTERM

CMD ["./main"]
