FROM golang:1.17-alpine AS builder
RUN apk add -U --no-cache \
    build-base \
    ca-certificates \
    git \
    sqlite \
    taglib-dev \
    alsa-lib-dev \
    zlib-dev \
    go
WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN GOOS=linux go build -o gonic cmd/gonic/gonic.go

FROM alpine:3.15
LABEL org.opencontainers.image.source https://github.com/sentriz/gonic
RUN apk add -U --no-cache \
    ffmpeg \
    ca-certificates \
    tzdata \
    tini

COPY --from=builder \
    /usr/lib/libgcc_s.so.1 \
    /usr/lib/libstdc++.so.6 \
    /usr/lib/libtag.so.1 \
    /usr/lib/
COPY --from=builder \
    /src/gonic \
    /bin/
VOLUME ["/cache", "/data", "/music", "/podcasts"]
EXPOSE 80
ENV TZ ""
ENV GONIC_DB_PATH /data/gonic.db
ENV GONIC_LISTEN_ADDR :80
ENV GONIC_MUSIC_PATH /music
ENV GONIC_PODCAST_PATH /podcasts
ENV GONIC_CACHE_PATH /cache
ENTRYPOINT ["/sbin/tini", "--"]
CMD ["gonic"]
