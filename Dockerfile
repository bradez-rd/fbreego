FROM golang:1.27-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fbreego ./cmd/fbreego

FROM alpine:3.22

ENV FBREEGO_PORT=3004

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S fbreego && adduser -S -G fbreego fbreego \
    && mkdir -p /var/lib/fbreego/sessions /var/lib/fbreego/media \
    && chown -R fbreego:fbreego /var/lib/fbreego
COPY --from=builder /out/fbreego /usr/local/bin/fbreego

USER fbreego
EXPOSE 3004

HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:${FBREEGO_PORT:-3004}/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/fbreego"]
