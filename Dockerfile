FROM golang:1.12-alpine AS base
RUN apk add -U --no-cache \
        build-base \
        ca-certificates \
        git \
        sqlite \
        taglib-dev
WORKDIR /src
COPY go.mod .
COPY go.sum .
ENV GO111MODULE=on
RUN go mod download

FROM base AS builder
WORKDIR /src
COPY . .
RUN ./_do_build_server && ./_do_build_scanner

FROM alpine
RUN apk add -U --no-cache \
	ffmpeg \
	ca-certificates
COPY --from=builder \
    /usr/lib/libgcc_s.so.1 \
    /usr/lib/libstdc++.so.6 \
    /usr/lib/libtag.so.1 \
    /usr/lib/
COPY --from=builder \
    /src/gonic \
    /src/gonicscan \
    /bin/
VOLUME ["/data", "/music", "/cache"]
EXPOSE 80
ENV GONIC_DB_PATH /data/gonic.db
ENV GONIC_LISTEN_ADDR :80
ENV GONIC_MUSIC_PATH /music
ENV GONIC_CACHE_PATH /cache
CMD ["gonic"]
