# Multi-stage build
FROM golang:1.24 AS builder

WORKDIR /src

# Download modules first
COPY go.mod go.sum ./
RUN go mod download

# Copy sources and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/corpstore ./cmd

# Final image
FROM alpine:3.18

COPY --from=builder /app/corpstore /app/corpstore

RUN adduser -D -g '' corpuser
USER corpuser

WORKDIR /app
VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["/app/corpstore"]
