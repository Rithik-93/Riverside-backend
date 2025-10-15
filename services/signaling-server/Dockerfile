# =============================================================================
# Build Stage
# =============================================================================
FROM golang:1.24.4-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY ../../protos /build/protos

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION:-dev} -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -trimpath \
    -o signaling-server \
    main.go

# =============================================================================
# Final Stage
# =============================================================================
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

COPY --from=builder /build/signaling-server /signaling-server

ENV TZ=UTC

USER 1000:1000

EXPOSE 8080

ENTRYPOINT ["/signaling-server"]

