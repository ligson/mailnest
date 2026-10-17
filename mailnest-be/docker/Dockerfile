ARG GO_BASE=golang:1.26-bookworm
ARG RUNTIME_BASE=debian:bookworm-slim
FROM ${GO_BASE} AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG APP_VERSION=
ARG VCS_REF=unknown
ARG BUILD_TIME=
RUN BUILD_TIME="${BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}" \
    && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build \
      -ldflags "-X mailnest-be/internal/buildinfo.Version=${APP_VERSION} -X mailnest-be/internal/buildinfo.Commit=${VCS_REF} -X mailnest-be/internal/buildinfo.BuildTime=${BUILD_TIME}" \
      -o /out/mailnest ./cmd/mailnest

FROM ${RUNTIME_BASE}

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/mailnest /app/mailnest

ENV MAILNEST_CONFIG=/app/config.yaml
EXPOSE 8080

ENTRYPOINT ["/app/mailnest"]
