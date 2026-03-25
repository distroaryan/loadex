FROM golang:alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o loadbalancer cmd/loadbalancer/main.go
RUN go build -o loadex cmd/loadex/main.go
RUN go build -o backend cmd/backend/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/loadbalancer /app/loadbalancer
COPY --from=builder /app/loadex /app/loadex
COPY --from=builder /app/backend /app/backend

# We will run loadbalancer or backend by overriding CMD in compose
CMD ["/app/loadbalancer"]
