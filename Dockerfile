# Build
ARG GO_VERSION=1.25
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

# Copy the rest
COPY . .

# Build the binary
ARG TARGETOS
ARG TARGETARCH
ENV CGO_ENABLED=0
RUN GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" -o /build/fxrates ./cmd

# Runtime
FROM debian:bookworm-slim

WORKDIR /app

# Install CA bundle (needed for TLS)
RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates \
 && update-ca-certificates \
 && rm -rf /var/lib/apt/lists/*

# Only the binary
COPY --from=build /build/fxrates /app/fxrates

# Server listens on 8080 by default
EXPOSE 8080

ENTRYPOINT ["./fxrates", "serve", "sql"]