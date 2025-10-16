# =============================================================================
# Build Stage
# =============================================================================
FROM golang:1.23.4-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY backend/protos /build/protos

COPY backend/services/signaling-server/. .
RUN sed -i 's|replace github.com/lakeside/backend/protos => ../../protos|replace github.com/lakeside/backend/protos => /build/protos|' go.mod
RUN go mod download && go mod verify

ARG VERSION=dev
ARG BUILD_TIME

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.buildTime=${BUILD_TIME}" \
    -trimpath \
    -o signaling-server \
    main.go

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
COPY --from=builder /build/signaling-server /signaling-server

ENV TZ=UTC

USER 1000:1000

EXPOSE 8080

ENTRYPOINT ["/signaling-server"]