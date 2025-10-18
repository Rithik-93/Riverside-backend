# =============================================================================
# Build Stage
# =============================================================================
FROM golang:1.23.0-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build/services/video-processor

COPY backend/protos /build/protos

COPY backend/services/video-processor/. .

RUN go mod download && go mod verify

ARG VERSION=dev
ARG BUILD_TIME

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" \
    -trimpath \
    -o video-processor \
    ./cmd/main.go

RUN echo "appuser:x:1000:1000:appuser:/:/sbin/nologin" > /etc/passwd.minimal && \
    echo "appuser:x:1000:" > /etc/group.minimal

# =============================================================================
# Final Stage
# =============================================================================
FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/passwd.minimal /etc/passwd
COPY --from=builder /etc/group.minimal /etc/group
COPY --from=builder /build/services/video-processor/video-processor /video-processor

ENV TZ=UTC

USER 1000:1000

ENTRYPOINT ["/video-processor"]

