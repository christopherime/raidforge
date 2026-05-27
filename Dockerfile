# Multi-stage build: compile the Go backend, ship a minimal non-root image.
# The frontend build stage is added in a later milestone (see TODO Phase 9).
ARG GO_VERSION=1.26

FROM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
# Copy the module sources (backend/ holds go.mod and the Go code).
COPY backend/ ./
RUN go mod download
# BUILD_REF is the version stamp; CI passes the short commit SHA.
ARG BUILD_REF=dev
ENV CGO_ENABLED=0
RUN go build -ldflags "-s -w -X main.version=${BUILD_REF}" -o /out/raidforge ./cmd/raidforge

FROM alpine:3
# ca-certificates for outbound HTTPS to the Blizzard / WCL / Raider.IO APIs (later milestones).
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 raidforge
USER raidforge
COPY --from=build /out/raidforge /usr/local/bin/raidforge
EXPOSE 8080
# busybox wget (bundled in alpine) hits the in-process health probe.
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null 2>&1 || exit 1
ENTRYPOINT ["raidforge"]
