# Build stage
FROM golang:1.24.3-alpine AS builder

WORKDIR /app

# Copy the monorepo content
COPY . .

# Build the binary
WORKDIR /app/apps/wallet-service
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Run stage
FROM alpine:3.19

WORKDIR /app

COPY --from=builder /app/apps/wallet-service/main .

CMD ["./main"]
