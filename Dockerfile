# --- Build stage ---
FROM golang:1.22-alpine AS builder
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /upnp-ui .

# --- Runtime stage ---
FROM alpine:3.20
RUN apk add --no-cache ca-certificates sqlite-libs tzdata
WORKDIR /app
COPY --from=builder /upnp-ui .
COPY web ./web
ENV TZ=Asia/Shanghai
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/app/upnp-ui"]