 <p align="center"><img width="500" src="https://github.com/sentriz/gonic/blob/master/.github/logo.png?raw=true"></p>
 <h4 align="center">FLOSS alternative to <a href="http://www.subsonic.org/">subsonic</a>, supporting its many clients</h4>
 <p align="center"><a href="http://hub.docker.com/r/sentriz/gonic"><img src="https://img.shields.io/docker/pulls/sentriz/gonic.svg"></a> <a href="https://microbadger.com/images/sentriz/gonic" title="Get your own image badge on microbadger.com"><img src="https://images.microbadger.com/badges/image/sentriz/gonic.svg"></a> <img src="https://img.shields.io/github/issues/sentriz/gonic.svg"> <img src="https://img.shields.io/github/issues-pr/sentriz/gonic.svg"></p>


 ## features

 - browsing by folder (keeping your full tree intact)  
 - browsing by tags (using [taglib](https://taglib.org/) - supports mp3, opus, flac, ape, m4a, wav, etc.)  
 - on-the-fly audio transcoding and caching (requires [ffmpeg](https://ffmpeg.org/)) (thank you [spijet](https://github.com/spijet/))
 - pretty fast scanning (with my library of ~27k tracks, initial scan takes about 10m, and about 5s after incrementally)  
 - multiple users, each with their own transcoding preferences, playlists, top tracks, top artists, etc.
 - [last.fm](https://www.last.fm/) scrobbling  
 - artist similarities and biographies from the last.fm api  
 - a web interface for configuration (set up last.fm, manage users, start scans, etc.)  
 - support for the [album-artist](https://mkoby.com/2007/02/18/artist-versus-album-artist/) tag, to not clutter your artist list with compilation album appearances  
 - written in [go](https://golang.org/), so lightweight and suitable for a raspberry pi, etc.  
 - newer salt and token auth  
 - tested on [dsub](https://f-droid.org/en/packages/github.daneren2005.dsub/), [jamstash](http://jamstash.com/), [sublime music](https://gitlab.com/sumner/sublime-music/), and [soundwaves](https://apps.apple.com/us/app/soundwaves/id736139596)  
 
 
## installation

the default login is **admin**/**admin**.  
password can then be changed from the web interface

```
$ apt install build-essential git sqlite libtag1-dev ffmpeg libasound-dev # for debian like
$ pacman -S base-devel git sqlite taglib ffmpeg alsa-lib                  # for arch like
$ go get go.senan.xyz/gonic/cmd/gonic
$ export PATH=$PATH:$HOME/go/bin
$ gonic -h # or see "configuration options below"
```

**note:** unfortunately if you do this above, you'll be compiling gonic locally on your machine
(if someone knows how I can statically link sqlite3 and taglib, please let me know so I can distribute static binaries)  

or else you can run in docker, available on dockerhub as `sentriz/gonic`

```yaml
# example docker-compose.yml

version: '3'
services:
  gonic:
    image: sentriz/gonic:latest
    environment:
    - TZ
    # optionally, see env vars below
    expose:
    - 80
    volumes:
    - ./data:/data             # gonic db etc
    - /path/to/music:/music:ro # your music
    - /path/to/cache:/cache    # transcode cache dir
```

then start with `docker-compose up -d`

## configuration options

|env var|command line arg|description|
|---|---|---|
|`GONIC_MUSIC_PATH`|`-music-path`|path to your music collection|
|`GONIC_CACHE_PATH`|`-cache-path`|**optional** path to store audio transcodes (*default* `/tmp/gonic_cache`)|
|`GONIC_DB_PATH`|`-db-path`|**optional** path to database file|
|`GONIC_LISTEN_ADDR`|`-listen-addr`|**optional** host and port to listen on (eg. `0.0.0.0:4747`, `127.0.0.1:4747`) (*default* `0.0.0.0:4747`)|
|`GONIC_PROXY_PREFIX`|`-proxy-prefix`|**optional** url path prefix to use if behind reverse proxy. eg `/gonic` (see example configs below)|
|`GONIC_SCAN_INTERVAL`|`-scan-interval`|**optional** interval (in minutes) to check for new music (automatic scanning disabled if omitted)|

## screenshots

||||||
|:-:|:-:|:-:|:-:|:-:|
![](https://raw.githubusercontent.com/sentriz/gonic/master/.github/scrot_1.png)|![](https://raw.githubusercontent.com/sentriz/gonic/master/.github/scrot_2.png)|![](https://raw.githubusercontent.com/sentriz/gonic/master/.github/scrot_3.png)|![](https://raw.githubusercontent.com/sentriz/gonic/master/.github/scrot_4.png)|![](https://raw.githubusercontent.com/sentriz/gonic/master/.github/scrot_5.png)|

## example nginx config with `GONIC_PROXY_PREFIX`

```nginx
  location /gonic/ {
      proxy_pass http://localhost:4747/;
      # set "Secure" cookie if using HTTPS
      proxy_cookie_path / "/; Secure";
      # set "X-Forwarded-Host" header for last.fm connection callback
      proxy_set_header X-Forwarded-Host $host;
  }
```
