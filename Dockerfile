# Stage 1: build the frontend once on the native build platform (static output).
FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
# Preserve the tracked placeholder so the embed dir exists before the build.
RUN mkdir -p /app/internal/web/dist
COPY internal/web/dist/index.html /app/internal/web/dist/index.html
RUN npm run build

# Stage 2: cross-compile the Go binary on the native build platform.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder
WORKDIR /app
ENV GOTOOLCHAIN=local
ENV CGO_ENABLED=0
RUN apk add --no-cache git ca-certificates
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/internal/web/dist ./internal/web/dist
ARG VERSION=docker
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} go build \
    -trimpath -ldflags="-s -w -X github.com/t0mer/bothan/internal/version.Version=${VERSION}" \
    -o /out/bothan ./cmd/bothan
# Pre-create the data dir so the scratch image has a writable SQLite location.
RUN mkdir -p /data

# Stage 3: minimal scratch runtime.
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /out/bothan /bothan
COPY --from=builder /data /data
ENV BOTHAN_DATABASE_PATH=/data/bothan.db
EXPOSE 8080
VOLUME ["/data"]
ENTRYPOINT ["/bothan"]
