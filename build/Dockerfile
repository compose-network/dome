###############################################
FROM --platform=$BUILDPLATFORM golang:1.25@sha256:e68f6a00e88586577fafa4d9cefad1349c2be70d21244321321c407474ff9bf2 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN if [ ! -f configs/config.yaml ]; then \
      cp configs/config.example.yaml configs/config.yaml; \
    fi

ARG TARGETOS TARGETARCH
ENV CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH

RUN go test -c ./test/ -o bin/dome

###############################################
FROM busybox AS busybox
RUN addgroup -S domegroup && adduser -S -G domegroup domeuser

###############################################
FROM debian:stable-slim@sha256:e0249870a90044494c01e74fbfc8b77ab14e6f47cece844d1e1737f7828a7e1e AS certs
RUN apt-get update && apt-get install -y ca-certificates

###############################################
FROM scratch

WORKDIR /app

COPY --from=builder /app/bin/dome .

COPY --from=busybox /etc/passwd /etc/passwd
COPY --from=busybox /etc/group /etc/group

COPY --from=certs /etc/ssl/certs /etc/ssl/certs
COPY --from=certs /etc/ssl/private /etc/ssl/private
COPY --from=certs /usr/share/ca-certificates /usr/share/ca-certificates

USER domeuser:domegroup

ENTRYPOINT ["/app/dome"]
CMD []