# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w" -o /out/elasticsearch-query-exporter ./cmd/elasticsearch-query-exporter

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/elasticsearch-query-exporter /elasticsearch-query-exporter

USER nonroot:nonroot
EXPOSE 9206
ENTRYPOINT ["/elasticsearch-query-exporter"]
