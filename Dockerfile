# Build the API server. The migrations are embedded in the binary, so the runtime image
# needs nothing but the binary itself and the demo examples.
FROM golang:1.22-alpine AS build
WORKDIR /src

# Dependencies first, so a source-only change does not re-download modules.
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
COPY migrations ./migrations

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/composer-server ./cmd/composer-server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/agentctl ./cmd/agentctl

FROM alpine:3.20
RUN apk add --no-cache ca-certificates curl && adduser -D -u 10001 composer
WORKDIR /app

COPY --from=build /out/composer-server /usr/local/bin/composer-server
COPY --from=build /out/agentctl /usr/local/bin/agentctl
COPY configs ./configs
COPY examples ./examples

USER composer
EXPOSE 8088

HEALTHCHECK --interval=10s --timeout=3s --start-period=20s --retries=5 \
  CMD curl -fsS http://localhost:8088/readyz || exit 1

ENTRYPOINT ["composer-server", "-config", "configs/config.yaml"]
