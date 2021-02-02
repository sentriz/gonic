<p align="center"><img width="500" src="https://github.com/sentriz/gonic/blob/master/.github/logo.png?raw=true"></p>
<h4 align="center">FLOSS alternative to <a href="http://www.subsonic.org/">subsonic</a>, supporting its many clients</h4>
<p align="center">
  <a href="http://hub.docker.com/r/sentriz/gonic"><img src="https://img.shields.io/docker/pulls/sentriz/gonic.svg"></a>
  <a href="https://microbadger.com/images/sentriz/gonic" title="Get your own image badge on microbadger.com"><img src="https://images.microbadger.com/badges/image/sentriz/gonic.svg"></a>
  <a href="https://github.com/sentriz/gonic/issues"><img src="https://img.shields.io/github/issues/sentriz/gonic.svg"></a>
  <a href="https://github.com/sentriz/gonic/pulls"><img src="https://img.shields.io/github/issues-pr/sentriz/gonic.svg"></a>
  <a href="https://github.com/sentriz/gonic/actions"><img src="https://img.shields.io/endpoint.svg?url=https%3A%2F%2Factions-badge.atrox.dev%2Fsentriz%2Fgonic%2Fbadge&label=build&logo=none"></a>
</p>


## features

- browsing by folder (keeping your full tree intact) [see here](#directory-structure)  
- browsing by tags (using [taglib](https://taglib.org/) - supports mp3, opus, flac, ape, m4a, wav, etc.)  
- on-the-fly audio transcoding and caching (requires [ffmpeg](https://ffmpeg.org/)) (thank you [spijet](https://github.com/spijet/))  
- jukebox mode (thank you [lxea](https://github.com/lxea/))  
- support for per-user podcasts (thank you [lxea](https://github.com/lxea/))  
- pretty fast scanning (with my library of ~27k tracks, initial scan takes about 10m, and about 5s after incrementally)  
- multiple users, each with their own transcoding preferences, playlists, top tracks, top artists, etc.  
- [last.fm](https://www.last.fm/) scrobbling  
- [listenbrainz](https://listenbrainz.org/) scrobbling (thank you [spezifisch](https://github.com/spezifisch), [lxea](https://github.com/lxea))
- artist similarities and biographies from the last.fm api  
- multiple genre support (see `GONIC_GENRE_SPLIT` to split tag strings on a character, eg. `;`, and browse them individually)  
- a web interface for configuration (set up last.fm, manage users, start scans, etc.)  
- support for the [album-artist](https://mkoby.com/2007/02/18/artist-versus-album-artist/) tag, to not clutter your artist list with compilation album appearances  
- written in [go](https://golang.org/), so lightweight and suitable for a raspberry pi, etc.  
- newer salt and token auth  
- tested on [dsub](https://f-droid.org/en/packages/github.daneren2005.dsub/), [jamstash](http://jamstash.com/), [sublime music](https://gitlab.com/sumner/sublime-music/), [soundwaves](https://apps.apple.com/us/app/soundwaves/id736139596), and [stmp](https://github.com/wildeyedskies/stmp)  


## installation

the default login is **admin**/**admin**.  
password can then be changed from the web interface

###  ...from source

```bash
$ apt install build-essential git sqlite libtag1-dev ffmpeg libasound-dev # for debian like
$ pacman -S base-devel git sqlite taglib ffmpeg alsa-lib                  # for arch like
$ go get go.senan.xyz/gonic/cmd/gonic
$ export PATH=$PATH:$HOME/go/bin
$ gonic -h # or see "configuration options below"
```

**note:** unfortunately if you do this above, you'll be compiling gonic locally on your machine
(if someone knows how I can statically link sqlite3 and taglib, please let me know so I can distribute static binaries) 

###  ...with docker `x86_64`

the image is available on dockerhub as [sentriz/gonic](https://hub.docker.com/r/sentriz/gonic) 

```yaml
# example docker-compose.yml

version: '2.4'
services:
  gonic:
    image: sentriz/gonic:latest
    environment:
    - TZ
    # optionally, see more env vars below
    expose:
    - 80
    volumes:
    - ./data:/data                # gonic db etc
    - /path/to/music:/music:ro    # your music
    - /path/to/podcasts:/podcasts # your music
    - /path/to/cache:/cache       # transcode / covers / etc cache dir

    # set the following two sections if you've enabled jukebox
    group_add:
    - audio
    devices:
    - /dev/snd:/dev/snd
```

then start with `docker-compose up -d`

###  ...with docker `arm / raspberry pi`

please see [ugeek/gonic](https://hub.docker.com/r/ugeek/gonic)

###  ...with systemd

example by @IUCCA, tested on Ubuntu 18.04

1. add a repository with the latest Version of golang and install the prerequisites

```bash
$ sudo add-apt-repository ppa:longsleep/golang-backports
$ sudo apt update && sudo apt upgrade
$ sudo apt install build-essential git sqlite libtag1-dev ffmpeg libasound-dev golang
```

2. download and compile gonic in the home directory  

```bash
$ go get go.senan.xyz/gonic/cmd/gonic
```

3. add a gonic user and create a directory for the server

```bash
$ sudo mkdir -p /var/gonic/
$ sudo adduser --disabled-login --gecos "" gonic
$ sudo mv go/bin/gonic /var/gonic/
$ sudo wget https://raw.githubusercontent.com/sentriz/gonic/master/contrib/config -O /var/gonic/config
$ sudo chmod -R 750 /var/gonic/
$ sudo chown -R gonic:gonic /var/gonic/
```

4. add your `music-path` to the config file

```bash
$ sudo nano /var/gonic/config
```

5. setup systemd service

```bash
$ sudo wget https://raw.githubusercontent.com/sentriz/gonic/master/contrib/gonic.service -O /etc/systemd/system/gonic.service
$ sudo systemctl daemon-reload
$ sudo systemctl enable --now gonic
```

###  ...elsewhere

[![](https://repology.org/badge/vertical-allrepos/gonic.svg)](https://repology.org/project/gonic/versions)

## configuration options

|env var|command line arg|description|
|---|---|---|
|`GONIC_MUSIC_PATH`|`-music-path`|path to your music collection|
|`GONIC_PODCAST_PATH`|`-podcast-path`|path to a podcasts directory|
|`GONIC_CACHE_PATH`|`-cache-path`|path to store audio transcodes, covers, etc|
|`GONIC_DB_PATH`|`-db-path`|**optional** path to database file|
|`GONIC_LISTEN_ADDR`|`-listen-addr`|**optional** host and port to listen on (eg. `0.0.0.0:4747`, `127.0.0.1:4747`) (*default* `0.0.0.0:4747`)|
|`GONIC_PROXY_PREFIX`|`-proxy-prefix`|**optional** url path prefix to use if behind reverse proxy. eg `/gonic` (see example configs below)|
|`GONIC_SCAN_INTERVAL`|`-scan-interval`|**optional** interval (in minutes) to check for new music (automatic scanning disabled if omitted)|
|`GONIC_JUKEBOX_ENABLED`|`-jukebox-enabled`|**optional** whether the subsonic [jukebox api](https://airsonic.github.io/docs/jukebox/) should be enabled|
|`GONIC_GENRE_SPLIT`|`-genre-split`|**optional** a string or character to split genre tags on for multi-genre support (eg. `;`)|

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

## directory structure

when browsing by folder, any arbitrary and nested folder layout is supported. with one exception: _albums must be contained in a single folder_  

please see [here](https://github.com/sentriz/gonic/issues/89) for more context  

```
music
├── drum and bass
│   └── Photek
│       └── (1997) Modus Operandi
│           ├── 01.10 The Hidden Camera.flac
│           ├── 02.10 Smoke Rings.flac
│           ├── 03.10 Minotaur.flac
│           └── folder.jpg
└── experimental
    └── Alan Vega
        ├── (1980) Alan Vega
        │   ├── 01.08 Jukebox Babe.flac
        │   ├── 02.08 Fireball.flac
        │   ├── 03.08 Kung Foo Cowboy.flac
        │   └── folder.jpg
        └── (1990) Deuce Avenue
            ├── 01.13 Body Bop Jive.flac
            ├── 02.13 Sneaker Gun Fire.flac
            ├── 03.13 Jab Gee.flac
            └── folder.jpg
```
