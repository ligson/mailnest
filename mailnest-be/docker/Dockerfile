FROM golang:1.26-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/mailnest ./cmd/mailnest

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/mailnest /app/mailnest

ENV MAILNEST_CONFIG=/app/config.yaml
EXPOSE 8080

ENTRYPOINT ["/app/mailnest"]
