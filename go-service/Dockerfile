FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build \
    -trimpath \
    -ldflags="-s -w -X main.version=${VERSION}" \
    -o /out/market-data \
    ./cmd/market-data

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /out/market-data /usr/local/bin/market-data

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/market-data"]
