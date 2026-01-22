# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /workspace

# Install dependencies
RUN apk add --no-cache git make ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
ARG VERSION=dev
ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w \
    -X 'github.com/zijiren233/sealos-state-metric/cmd/sealos-state-metric/app.Version=${VERSION}' \
    -X 'github.com/zijiren233/sealos-state-metric/cmd/sealos-state-metric/app.GitCommit=${GIT_COMMIT}' \
    -X 'github.com/zijiren233/sealos-state-metric/cmd/sealos-state-metric/app.BuildDate=${BUILD_DATE}'" \
    -o /workspace/bin/sealos-state-metric \
    ./cmd/sealos-state-metric

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Install ca-certificates for HTTPS
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 65534 sealos && \
    adduser -u 65534 -G sealos -s /bin/sh -D sealos

# Copy binary from builder
COPY --from=builder /workspace/bin/sealos-state-metric /app/sealos-state-metric

# Copy default config
COPY config/default.yaml /etc/sealos-state-metric/config.yaml

# Change ownership
RUN chown -R sealos:sealos /app /etc/sealos-state-metric

# Switch to non-root user
USER sealos

# Expose metrics port
EXPOSE 9090

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD ["/app/sealos-state-metric", "--help"] || exit 1

# Run the binary
ENTRYPOINT ["/app/sealos-state-metric"]
CMD ["--config=/etc/sealos-state-metric/config.yaml"]
