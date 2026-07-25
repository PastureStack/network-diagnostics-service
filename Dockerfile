FROM golang:1.26.5-bookworm AS build

ARG VERSION=dev
ARG SOURCE_DATE_EPOCH=0
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
COPY package ./package
RUN go test ./... \
    && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
      -trimpath \
      -ldflags="-s -w -X main.version=${VERSION}" \
      -o /out/network-diagnostics-service \
      ./cmd/network-diagnostics-service \
    && mkdir -p /rootfs/etc/ssl/certs /rootfs/var/lib/pasturestack-network-diagnostics \
    && cp /etc/ssl/certs/ca-certificates.crt /rootfs/etc/ssl/certs/ca-certificates.crt \
    && cp /out/network-diagnostics-service /rootfs/network-diagnostics-service \
    && cp package/data/.keep /rootfs/var/lib/pasturestack-network-diagnostics/.keep \
    && chmod 0755 /rootfs /rootfs/etc /rootfs/etc/ssl /rootfs/etc/ssl/certs /rootfs/network-diagnostics-service \
    && chmod 0700 /rootfs/var /rootfs/var/lib /rootfs/var/lib/pasturestack-network-diagnostics \
    && chmod 0644 /rootfs/etc/ssl/certs/ca-certificates.crt \
    && chmod 0600 /rootfs/var/lib/pasturestack-network-diagnostics/.keep \
    && find /rootfs -exec touch --no-dereference --date="@${SOURCE_DATE_EPOCH}" {} +

FROM scratch

ARG VERSION=dev
ARG SOURCE_DATE_EPOCH=0
LABEL org.opencontainers.image.title="PastureStack Network Diagnostics Service" \
      org.opencontainers.image.description="Authenticated, bounded network diagnostics bundle service" \
      org.opencontainers.image.source="https://github.com/PastureStack/network-diagnostics-service" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.licenses="Apache-2.0"
COPY --from=build --chown=65532:65532 /rootfs/ /
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/network-diagnostics-service", "serve"]
