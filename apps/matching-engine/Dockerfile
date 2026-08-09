# Build stage
FROM golang:1.24.3-alpine AS builder

WORKDIR /app

# Copy the monorepo content
COPY . .

# Build the binary
WORKDIR /app/apps/matching-engine
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# Run stage
FROM alpine:3.19

RUN addgroup -S velyxgroup && adduser -S velyxuser -G velyxgroup

WORKDIR /app

COPY --from=builder /app/apps/matching-engine/main .

USER velyxuser

CMD ["./main"]
