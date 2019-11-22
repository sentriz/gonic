 <p align="center"><img width="500" src="https://github.com/sentriz/gonic/blob/master/.github/logo.png?raw=true"></p>
 <h4 align="center">FLOSS alternative to <a href="http://www.subsonic.org/">subsonic</a>, supporting its many clients</h4>
 <p align="center"><a href="http://hub.docker.com/r/sentriz/gonic"><img src="https://img.shields.io/docker/pulls/sentriz/gonic.svg"></a> <a href="https://microbadger.com/images/sentriz/gonic" title="Get your own image badge on microbadger.com"><img src="https://images.microbadger.com/badges/image/sentriz/gonic.svg"></a> <img src="https://img.shields.io/github/issues/sentriz/gonic.svg"> <img src="https://img.shields.io/github/issues-pr/sentriz/gonic.svg"></p>


 ## features

 - browsing by folder (keeping your full tree intact)  
 - browsing by tags (using [taglib](https://taglib.org/) - supports mp3, opus, flac, ape, m4a, wav, etc.)  
 - pretty fast scanning (with my library of ~27k tracks, initial scan takes about 10m, and about 5s after incrementally)  
 - last.fm scrobbling  
 - multiple users  
 - a web interface for configuration (set up last.fm, manage users, start scans, etc.)  
 - newer salt and token auth  
 - tested on [dsub](https://f-droid.org/en/packages/github.daneren2005.dsub/) and [jamstash](http://jamstash.com/)  
 
 
## installation

The default login is **admin**/**admin**.

```
$ apt install sqlite tablib libtag1-dev
$ go get senan.xyz/g/gonic/cmd/gonic
$ gonic -h
```

or with docker, available on dockerhub as `sentriz/gonic`

```yaml
  gonic:
    image: sentriz/gonic:latest
    environment:
    - TZ
    - GONIC_MUSIC_PATH=/music
    - GONIC_DB_PATH=/data/db.db
    - GONIC_LISTEN_ADDR=:80
    expose:
    - 80
    volumes:
    - ./data:/data
    - ${YOUR_MUSIC}:/music:ro
```

## screenshots

<p align="center">
<p float="left">
    <img width="400" src="https://github.com/sentriz/gonic/raw/master/.github/scrot_2.png">
    <img width="400" src="https://github.com/sentriz/gonic/raw/master/.github/scrot_3.png">
</p>
</p>
