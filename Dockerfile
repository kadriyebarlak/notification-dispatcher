# Stage 1 — build
FROM golang:1.24-alpine AS builder

WORKDIR /app

# copy dependency files first — Docker layer cache
COPY go.mod go.sum ./
RUN go mod download

# copy source and build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

# Stage 2 — runtime
FROM alpine:3.20

WORKDIR /app

# copy binary from builder
COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]