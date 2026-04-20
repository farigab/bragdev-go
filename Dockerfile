# Multi-stage Dockerfile for Go (example)
FROM golang:1.21-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags='-s -w' -o /app/bragdoc ./cmd/bragdoc

FROM alpine:3.18
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /app/bragdoc /app/bragdoc
COPY --from=build /src/db/migrations /app/db/migrations
EXPOSE 8080
ENTRYPOINT ["/app/bragdoc"]
