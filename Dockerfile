# Build the Annales Go application.
FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/annales ./cmd/server

# Minimal runtime image.
FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && adduser -D -H -u 10001 annales

COPY --from=builder /out/annales /usr/local/bin/annales

USER 10001:10001

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/annales"]
