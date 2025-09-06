FROM alpine:3.22 AS builder-taglib
WORKDIR /tmp
COPY alpine/taglib/APKBUILD .
RUN apk update && \
    apk add --no-cache abuild doas && \
    echo "permit nopass root" > /etc/doas.conf && \
    abuild-keygen -a -n -i && \
    REPODEST=/pkgs abuild -F -r

FROM golang:1.25-alpine3.22 AS builder
RUN apk add -U --no-cache \
    build-base \
    ca-certificates \
    git \
    sqlite \
    zlib-dev \
    go

# TODO: delete this block when taglib v2 is on alpine packages
COPY --from=builder-taglib /pkgs/*/*.apk /pkgs/
RUN apk add --no-cache --allow-untrusted /pkgs/*

WORKDIR /src
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN GOOS=linux go build -o gonic cmd/gonic/gonic.go

FROM alpine:3.22
LABEL org.opencontainers.image.source=https://github.com/sentriz/gonic
RUN apk add -U --no-cache \
    ffmpeg \
    mpv \
    ca-certificates \
    tzdata \
    tini \
    shared-mime-info

COPY --from=builder \
    /usr/lib/libgcc_s.so.1 \
    /usr/lib/libstdc++.so.6 \
    /usr/lib/libtag.so.2 \
    /usr/lib/
COPY --from=builder \
    /src/gonic \
    /bin/
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
