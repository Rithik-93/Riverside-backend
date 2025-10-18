FROM golang:1.24-alpine

WORKDIR /app

# Install Air for hot reloading (v1.52.3 is compatible with Go 1.24)
RUN go install github.com/air-verse/air@v1.52.3

# Copy protos (needed for go mod replace)
COPY protos /protos

# Copy go mod files
COPY services/signaling-server/go.mod services/signaling-server/go.sum ./
RUN go mod download

# Copy source code
COPY services/signaling-server/ .

# Create Air config if it doesn't exist
RUN if [ ! -f .air.toml ]; then air init; fi

# Expose port
EXPOSE 8080

# Run Air for hot reloading
CMD ["air", "-c", ".air.toml"]

