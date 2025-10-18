FROM golang:1.24-alpine

WORKDIR /app

RUN go install github.com/air-verse/air@v1.52.3

# Copy protos (needed for go mod replace)
COPY protos /protos

# Copy go mod files
COPY services/video-processor/go.mod services/video-processor/go.sum ./
RUN go mod download

# Copy source code
COPY services/video-processor/ .

# Create Air config if it doesn't exist
RUN if [ ! -f .air.toml ]; then air init; fi

# Run Air for hot reloading
CMD ["air", "-c", ".air.toml"]

