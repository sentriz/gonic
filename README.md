<p align="center"><img width="500" src="https://github.com/sentriz/gonic/blob/master/.github/logo.png?raw=true"></p>
<h4 align="center">FLOSS alternative to <a href="http://www.subsonic.org/">subsonic</a>, supporting its many clients</h4>
<p align="center">
  <a href="http://hub.docker.com/r/sentriz/gonic"><img src="https://img.shields.io/docker/pulls/sentriz/gonic.svg"></a>
  <a href="https://github.com/sentriz/gonic/issues"><img src="https://img.shields.io/github/issues/sentriz/gonic.svg"></a>
  <a href="https://github.com/sentriz/gonic/pulls"><img src="https://img.shields.io/github/issues-pr/sentriz/gonic.svg"></a>
  <a href="https://github.com/sentriz/gonic/actions"><img src="https://img.shields.io/endpoint.svg?url=https%3A%2F%2Factions-badge.atrox.dev%2Fsentriz%2Fgonic%2Fbadge&label=build&logo=none"></a>
</p>

<p align="center">
  <b>libera.chat</b> <a href="ircs://irc.libera.chat/#gonic" title="#gonic on Libera Chat">#gonic</a>
</p>

## features

- browsing by folder (keeping your full tree intact) [see here](#directory-structure)
- browsing by tags (using [taglib](https://taglib.org/) - supports mp3, opus, flac, ape, m4a, wav, etc.)
- on-the-fly audio transcoding and caching (requires [ffmpeg](https://ffmpeg.org/)) (thank you [spijet](https://github.com/spijet/))
- jukebox mode (thank you [lxea](https://github.com/lxea/))
- support for podcasts (thank you [lxea](https://github.com/lxea/))
- pretty fast scanning (with my library of ~27k tracks, initial scan takes about 10m, and about 5s after incrementally)
- multiple users, each with their own transcoding preferences, playlists, top tracks, top artists, etc.
- [last.fm](https://www.last.fm/) scrobbling
- [listenbrainz](https://listenbrainz.org/) scrobbling (thank you [spezifisch](https://github.com/spezifisch), [lxea](https://github.com/lxea))
- artist similarities and biographies from the last.fm api
- multiple genre support (see `GONIC_GENRE_SPLIT` to split tag strings on a character, eg. `;`, and browse them individually)
- a web interface for configuration (set up last.fm, manage users, start scans, etc.)
- support for the [album-artist](https://mkoby.com/2007/02/18/artist-versus-album-artist/) tag, to not clutter your artist list with compilation album appearances
- written in [go](https://golang.org/), so lightweight and suitable for a raspberry pi, etc. (see ARM images below)
- newer salt and token auth
- tested on [airsonic-refix](https://github.com/tamland/airsonic-refix), [symfonium](https://symfonium.app), [dsub](https://f-droid.org/en/packages/github.daneren2005.dsub/), [jamstash](http://jamstash.com/),
  [sublime music](https://github.com/sublime-music/sublime-music), [soundwaves](https://apps.apple.com/us/app/soundwaves/id736139596),
  [stmp](https://github.com/wildeyedskies/stmp), [strawberry](https://www.strawberrymusicplayer.org/), and [ultrasonic](https://gitlab.com/ultrasonic/ultrasonic)

## installation

the default login is **admin**/**admin**.  
password can then be changed from the web interface

### ...from source

<https://github.com/sentriz/gonic/wiki/installation#from-source>

### ...with docker

<https://github.com/sentriz/gonic/wiki/installation#with-docker>

### ...with systemd

<https://github.com/sentriz/gonic/wiki/installation#with-systemd>

### ...elsewhere

[![](https://repology.org/badge/vertical-allrepos/gonic.svg)](https://repology.org/project/gonic/versions)

## configuration options

| env var                        | command line arg          | description                                                                                                 |
| ------------------------------ | ------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `GONIC_MUSIC_PATH`             | `-music-path`             | path to your music collection (see also multi-folder support below)                                         |
| `GONIC_PODCAST_PATH`           | `-podcast-path`           | path to a podcasts directory                                                                                |
| `GONIC_CACHE_PATH`             | `-cache-path`             | path to store audio transcodes, covers, etc                                                                 |
| `GONIC_DB_PATH`                | `-db-path`                | **optional** path to database file                                                                          |
| `GONIC_PLAYLIST_PATH`          | `-playlist-path`          | **optional** path to a directory containing `.m3u` playlists                                                |
| `GONIC_HTTP_LOG`               | `-http-log`               | **optional** http request logging, enabled by default                                                       |
| `GONIC_LISTEN_ADDR`            | `-listen-addr`            | **optional** host and port to listen on (eg. `0.0.0.0:4747`, `127.0.0.1:4747`) (_default_ `0.0.0.0:4747`)   |
| `GONIC_TLS_CERT`               | `-tls-cert`               | **optional** path to a TLS cert (enables HTTPS listening)                                                   |
| `GONIC_TLS_KEY`                | `-tls-key`                | **optional** path to a TLS key (enables HTTPS listening)                                                    |
| `GONIC_PROXY_PREFIX`           | `-proxy-prefix`           | **optional** url path prefix to use if behind reverse proxy. eg `/gonic` (see example configs below)        |
| `GONIC_SCAN_INTERVAL`          | `-scan-interval`          | **optional** interval (in minutes) to check for new music (automatic scanning disabled if omitted)          |
| `GONIC_SCAN_AT_START_ENABLED`  | `-scan-at-start-enabled`  | **optional** whether to perform an initial scan at startup                                                  |
| `GONIC_SCAN_WATCHER_ENABLED`   | `-scan-watcher-enabled`   | **optional** whether to watch file system for new music and rescan                                          |
| `GONIC_JUKEBOX_ENABLED`        | `-jukebox-enabled`        | **optional** whether the subsonic [jukebox api](https://airsonic.github.io/docs/jukebox/) should be enabled |
| `GONIC_JUKEBOX_MPV_EXTRA_ARGS` | `-jukebox-mpv-extra-args` | **optional** extra command line arguments to pass to the jukebox mpv daemon                                 |
| `GONIC_PODCAST_PURGE_AGE`      | `-podcast-purge-age`      | **optional** age (in days) to purge podcast episodes if not accessed                                        |
| `GONIC_GENRE_SPLIT`            | `-genre-split`            | **optional** a string or character to split genre tags on for multi-genre support (eg. `;`)                 |

## screenshots

|                                                                                 |                                                                                 |                                                                                 |                                                                                 |                                                                                 |
| :-----------------------------------------------------------------------------: | :-----------------------------------------------------------------------------: | :-----------------------------------------------------------------------------: | :-----------------------------------------------------------------------------: | :-----------------------------------------------------------------------------: |
| ![](https://raw.githubusercontent.com/sentriz/gonic/master/.github/scrot_1.png) | ![](https://raw.githubusercontent.com/sentriz/gonic/master/.github/scrot_2.png) | ![](https://raw.githubusercontent.com/sentriz/gonic/master/.github/scrot_3.png) | ![](https://raw.githubusercontent.com/sentriz/gonic/master/.github/scrot_4.png) | ![](https://raw.githubusercontent.com/sentriz/gonic/master/.github/scrot_5.png) |

## multiple folders support (v0.15+)

gonic supports multiple music folders. this can be handy if you have your music separated by albums, compilations, singles. or maybe 70s, 80s, 90s. whatever.

if you're running gonic with the command line, stack the `-music-path` arg

```shell
$ gonic -music-path /path/to/albums -music-path /path/to/compilations
```

if you're running gonic with ENV_VARS, or docker, try separate with a comma

```shell
GONIC_MUSIC_PATH=/path/to/albums,/path/to/compilations
```

if you're running gonic with the config file, you can repeat the `music-path` option

```shell
music-path /path/to/albums
music-path /path/to/compilations
```

after that, most subsonic clients should allow you to select which music folder to use.
queries like show me "recently played compilations" or "recently added albums" are possible for example.

## directory structure

when browsing by folder, any arbitrary and nested folder layout is supported, with the following caveats:

- Files from the same album must all be in the same folder
- All files in a folder must be from the same album

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
