# syntax=docker/dockerfile:1
FROM golang:1.26-bookworm AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X github.com/downdawn/goba-slim/internal/version.Version=${VERSION} -X github.com/downdawn/goba-slim/internal/version.Commit=${COMMIT} -X github.com/downdawn/goba-slim/internal/version.BuildTime=${BUILD_TIME}" \
    -o /out/goba ./cmd/goba

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /out/goba /goba

EXPOSE 8000

USER nonroot:nonroot

ENTRYPOINT ["/goba", "serve", "--config", "/etc/goba/config.yaml"]
