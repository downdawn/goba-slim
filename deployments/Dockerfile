# syntax=docker/dockerfile:1
FROM golang:1.26.5-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
ARG DIRTY=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X github.com/downdawn/goba-slim/internal/version.Version=${VERSION} -X github.com/downdawn/goba-slim/internal/version.Commit=${COMMIT} -X github.com/downdawn/goba-slim/internal/version.BuildTime=${BUILD_TIME} -X github.com/downdawn/goba-slim/internal/version.Dirty=${DIRTY}" \
    -o /out/goba ./cmd/goba

# 只读根文件系统下，Docker 不能创建文件挂载和 Secret 挂载的目标路径。
RUN mkdir -p /out/rootfs/etc/goba /out/rootfs/run/secrets /out/rootfs/var/lib/goba/uploads /out/rootfs/tmp \
    && touch /out/rootfs/etc/goba/config.yaml \
    && touch /out/rootfs/run/secrets/database_password /out/rootfs/run/secrets/redis_password /out/rootfs/run/secrets/auth_private_key

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/goba /goba
COPY --from=build --chown=65532:65532 /out/rootfs/ /

EXPOSE 8000

USER nonroot:nonroot

ENTRYPOINT ["/goba"]
CMD ["serve"]
