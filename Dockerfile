FROM golang:1.12-alpine AS builder
WORKDIR /src
COPY . .
RUN \
    apk add taglib-dev sqlite build-base git && \
    ./_do_build_server && \
    ./_do_build_scanner && \
    apk del build-base && \
    mv ./gonic ./gonicscan /bin/
VOLUME ["/data", "/music"]
EXPOSE 80
ENV GONIC_LISTEN_ADDR :80
ENV GONIC_DB_PATH /data/gonic.db
ENV GONIC_MUSIC_PATH /music
CMD ["gonic"]
