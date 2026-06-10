# syntax=docker/dockerfile:1

# ---- build stage ---------------------------------------------------------
# Pinned to match the toolchain in go.mod and CI.
FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache dependencies before copying the rest of the source.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Static, stripped binary. CGO is disabled so the result runs on a scratch-like
# distroless base.
ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ---- runtime stage -------------------------------------------------------
# distroless/static:nonroot runs as an unprivileged user (uid 65532) and ships
# no shell or package manager, minimising attack surface.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
COPY --from=build /out/api /app/api

EXPOSE 8080
USER nonroot:nonroot

# Self-probe via the binary's healthcheck subcommand (no shell in distroless).
HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/app/api", "healthcheck"]

ENTRYPOINT ["/app/api"]
