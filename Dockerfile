FROM golang:1.24-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/metric-api ./cmd/metric-api
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/sanitize-history ./cmd/sanitize-history

FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/metric-api /app/metric-api
COPY --from=build /out/sanitize-history /app/sanitize-history
RUN mkdir -p /data

EXPOSE 30812
CMD ["/app/metric-api"]
