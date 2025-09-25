# syntax=docker/dockerfile:1
FROM --platform=$BUILDPLATFORM golang:1.25-alpine3.22 AS builder
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN  \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /out/ ./cmd/...

FROM alpine:3.22
LABEL org.opencontainers.image.source=https://github.com/sentriz/gonic
RUN apk add -U --no-cache \
    ffmpeg \
    mpv \
    ca-certificates \
    tzdata \
    tini \
    shared-mime-info
COPY --from=builder /out/* /usr/local/bin/
VOLUME ["/cache", "/data", "/music", "/podcasts"]
EXPOSE 80
ENV TZ=
ENV GONIC_DB_PATH=/data/gonic.db
ENV GONIC_LISTEN_ADDR=:80
ENV GONIC_MUSIC_PATH=/music
ENV GONIC_PODCAST_PATH=/podcasts
ENV GONIC_CACHE_PATH=/cache
ENV GONIC_PLAYLISTS_PATH=/playlists
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["gonic"]
