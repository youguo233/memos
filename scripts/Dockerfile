FROM --platform=$BUILDPLATFORM golang:1.27.2-alpine AS backend
WORKDIR /backend-build

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files and download dependencies (cached layer)
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy source code (use .dockerignore to exclude unnecessary files)
COPY . .

# Please build frontend first, so that the static files are available.
# Refer to `pnpm release` in package.json for the build command.
ARG TARGETOS TARGETARCH VERSION COMMIT=unknown
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    : "${VERSION:?Pass VERSION from scripts/release_version.sh development-version or a release tag}" && \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build \
      -trimpath \
      -ldflags="-s -w -X github.com/usememos/memos/internal/version.Version=${VERSION} -X github.com/usememos/memos/internal/version.Commit=${COMMIT} -extldflags '-static'" \
      -tags netgo,osusergo \
      -o memos \
      ./cmd/memos

# Pandoc (official static Linux binary) for DOCX export. Available for amd64
# and arm64; on other architectures the stage leaves an empty directory and
# Memos falls back to its built-in pure-Go DOCX generator.
FROM alpine:3.21 AS pandoc
ARG TARGETARCH PANDOC_VERSION=3.6.4
RUN mkdir -p /out && \
    case "$TARGETARCH" in amd64|arm64) \
      wget -qO- "https://github.com/jgm/pandoc/releases/download/${PANDOC_VERSION}/pandoc-${PANDOC_VERSION}-linux-${TARGETARCH}.tar.gz" \
        | tar -xz -C /tmp && mv /tmp/pandoc-${PANDOC_VERSION}/bin/pandoc /out/pandoc && /out/pandoc --version ;; \
      *) echo "pandoc: no static build for $TARGETARCH, skipping" ;; \
    esac

# Use minimal Alpine with security updates
FROM alpine:3.21 AS monolithic

# Install runtime dependencies and create non-root user in single layer
RUN apk add --no-cache tzdata ca-certificates su-exec && \
    addgroup -g 10001 -S nonroot && \
    adduser -u 10001 -S -G nonroot -h /var/opt/memos nonroot && \
    mkdir -p /var/opt/memos /usr/local/memos && \
    chown -R nonroot:nonroot /var/opt/memos

# Copy binary and entrypoint to /usr/local/memos
COPY --from=backend /backend-build/memos /usr/local/memos/memos
COPY --from=pandoc /out/ /usr/local/bin/
COPY --from=backend --chmod=755 /backend-build/scripts/entrypoint.sh /usr/local/memos/entrypoint.sh

# Run as root to fix permissions, entrypoint will drop to nonroot
USER root

# Set working directory to the writable volume
WORKDIR /var/opt/memos

# Data directory
VOLUME /var/opt/memos

ENV TZ="UTC" \
    MEMOS_PORT="5230"

EXPOSE 5230

ENTRYPOINT ["/usr/local/memos/entrypoint.sh", "/usr/local/memos/memos"]
