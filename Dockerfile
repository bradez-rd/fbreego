FROM golang:1.27-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/fbreego ./cmd/fbreego

FROM alpine:3.22

RUN addgroup -S fbreego && adduser -S -G fbreego fbreego \
    && mkdir -p /var/lib/fbreego/sessions /var/lib/fbreego/media \
    && chown -R fbreego:fbreego /var/lib/fbreego
COPY --from=builder /out/fbreego /usr/local/bin/fbreego

USER fbreego
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/fbreego"]
