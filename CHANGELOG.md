# Changelog

### [0.16.3](https://www.github.com/sentriz/gonic/compare/v0.16.2...v0.16.3) (2024-03-09)


### Features

* bump audiotags fork to support taglib v2 ([29c5397](https://www.github.com/sentriz/gonic/commit/29c5397dae82017e24347afe65e9bbf9be10a494))
* **lastfm:** autocorrect artist and album name misspellings when fetching info ([2878b88](https://www.github.com/sentriz/gonic/commit/2878b88aeee8bdf7a2e45520298422b883d8ab24)), closes [#472](https://www.github.com/sentriz/gonic/issues/472)
* **lastfm:** strip copyright text from albumInfo/artistInfo responses ([aa82b94](https://www.github.com/sentriz/gonic/commit/aa82b944b794ac07b41a34aeb2b4cc365a2666ef))
* **listenbrainz:** submit release MBID ([#443](https://www.github.com/sentriz/gonic/issues/443)) ([552aa3a](https://www.github.com/sentriz/gonic/commit/552aa3afb138a125f15ab161a1a06cbe6c68a762))
* replace ff with with flagconf ([3ada74c](https://www.github.com/sentriz/gonic/commit/3ada74c4db61c90ba428a071282fdacfd038cfc0))
* **subsonic:** expose track musicbrainz id ([f98b232](https://www.github.com/sentriz/gonic/commit/f98b2326da31c15192f1c3b4bb17dcbfa59a058a))
* **subsonic:** prefer tagged musicbrainz ID over lastfm in getAlbumInfo ([422c684](https://www.github.com/sentriz/gonic/commit/422c684f44369a0c85cb1a825e3d480ee157ca0b)), closes [#444](https://www.github.com/sentriz/gonic/issues/444)
* **subsonic:** return `changed` field in playlist response ([8b4fc04](https://www.github.com/sentriz/gonic/commit/8b4fc04d3e7a22ba5beb3d682ea13b541c99d2bb)), closes [#455](https://www.github.com/sentriz/gonic/issues/455)
* **subsonic:** return `playCount` in album responses ([ec55f3b](https://www.github.com/sentriz/gonic/commit/ec55f3b22a8c689cbd2305965b3c538f5c2bf25f)), closes [#458](https://www.github.com/sentriz/gonic/issues/458)
* **subsonic:** return an error if maxBitRate requested with no user transcode preferences set ([88e58c0](https://www.github.com/sentriz/gonic/commit/88e58c055a2b1259d0c68618b943b4a319855b15))
* **subsonic:** return http form post opensubsonic extension key ([e8ae1c1](https://www.github.com/sentriz/gonic/commit/e8ae1c1d406c7013b2a739dd13bac3076bde641f))
* upgrade to ff v4 ([4600ee1](https://www.github.com/sentriz/gonic/commit/4600ee1cbb380bb7a9d255cbd51bd746e021eb63)), closes [#473](https://www.github.com/sentriz/gonic/issues/473)


### Bug Fixes

* **ci:** manually add taglib v2 APKBUILD ([51fa0ba](https://www.github.com/sentriz/gonic/commit/51fa0baac39577b335a3f5c064be81cf9293e9f0))
* **db:** add double index for right side of unique compound indexes ([d640a9f](https://www.github.com/sentriz/gonic/commit/d640a9fc065bd3908968abacbd5ac080331c3e25)), closes [#426](https://www.github.com/sentriz/gonic/issues/426)
* **docker:** fix sqlite3 musl build ([433829d](https://www.github.com/sentriz/gonic/commit/433829dc4f43f3be83a99cb54099be4f781dcf7d))
* **listenbrainz:** set track length submission and include submission client details ([#424](https://www.github.com/sentriz/gonic/issues/424)) ([b27c02f](https://www.github.com/sentriz/gonic/commit/b27c02fc894510b714c129e898fd0e6792d017b4))
* **playlist:** return new playlist id for createPlaylist ([314e963](https://www.github.com/sentriz/gonic/commit/314e9632d72dd7cd68044f9d69295123fff78f80)), closes [#464](https://www.github.com/sentriz/gonic/issues/464)
* **podcast:** collect all episode errors when adding new podcast ([2f109f1](https://www.github.com/sentriz/gonic/commit/2f109f1982dea78dbdfe786cafa3c6718138b66e)), closes [#430](https://www.github.com/sentriz/gonic/issues/430)
* **podcast:** slightly more robust downloading and concurrency ([#433](https://www.github.com/sentriz/gonic/issues/433)) ([f34cd2e](https://www.github.com/sentriz/gonic/commit/f34cd2e213f9be93ae3db4ad32e7e927fd15c618))
* **scanner:** clean up orphaned album genres when dir still exists without tracks ([19ebd45](https://www.github.com/sentriz/gonic/commit/19ebd4540f9e47c521cc70587457a770110495a5)), closes [#466](https://www.github.com/sentriz/gonic/issues/466)
* **scanner:** gracefully handle multi value tag delim splits with adjacent delimiters ([eb79cec](https://www.github.com/sentriz/gonic/commit/eb79cecc44628d3e24b2196e1d680f2b1ba15f97)), closes [#448](https://www.github.com/sentriz/gonic/issues/448)
* **specid:** match music dirs with trailing slash ([#439](https://www.github.com/sentriz/gonic/issues/439)) ([e63ee96](https://www.github.com/sentriz/gonic/commit/e63ee9687e1cb4b76faa9ffe10bf99b4640ddd43))
* **subsonic:** always return playlist duration  ([87943ea](https://www.github.com/sentriz/gonic/commit/87943ea863f5e882ff4acfe329d6dde674ac630b)), closes [#457](https://www.github.com/sentriz/gonic/issues/457)
* **subsonic:** fix getAvatar user request comparison ([#469](https://www.github.com/sentriz/gonic/issues/469)) ([2949b4c](https://www.github.com/sentriz/gonic/commit/2949b4c86715d9eebfe5bf116c8df7f0a5875eb3))
* **subsonic:** return error code 70 for not found errors in more places ([42dbfa7](https://www.github.com/sentriz/gonic/commit/42dbfa7a85a25fcb80f2936ac7949d50ed1bfbf8)), closes [#454](https://www.github.com/sentriz/gonic/issues/454)

### [0.16.2](https://www.github.com/sentriz/gonic/compare/v0.16.1...v0.16.2) (2023-11-30)


### Features

* set global http timeouts except for streaming endpoints ([2edb1b8](https://www.github.com/sentriz/gonic/commit/2edb1b8eda649671aa4b0534ec8065bde65803e3)), closes [#411](https://www.github.com/sentriz/gonic/issues/411)


### Bug Fixes

* **admin:** don't start with empty session key ([dd0f6b3](https://www.github.com/sentriz/gonic/commit/dd0f6b3650426a06218bff875301471b92c6f03e)), closes [#414](https://www.github.com/sentriz/gonic/issues/414)
* **jukebox:** make sure we clean up "seekable" event listener ([b199bc1](https://www.github.com/sentriz/gonic/commit/b199bc104e90eaf760563b6efca409ccd9618788)), closes [#411](https://www.github.com/sentriz/gonic/issues/411)
* **jukebox:** restore play index only when incoming new track has index >0 ([82c3c5b](https://www.github.com/sentriz/gonic/commit/82c3c5baef5a5145902cd96e1a14d6d3fd50320f)), closes [#411](https://www.github.com/sentriz/gonic/issues/411)
* **subsonic:** return empty opensubsonic fields ([5022500](https://www.github.com/sentriz/gonic/commit/5022500b307e746f2ff0426b07c4e776873c880a))

### [0.16.1](https://www.github.com/sentriz/gonic/compare/v0.16.0...v0.16.1) (2023-11-08)


### Features

* add more and unify stats ([2fdc1f4](https://www.github.com/sentriz/gonic/commit/2fdc1f41a25435f2c543ea114dd366c3c3d8394f))
* store and expose individual track artists ([c1a34dc](https://www.github.com/sentriz/gonic/commit/c1a34dc0219b83300b427e66ef388f27a6186c9f))
* **subsonic:** add getAlbumInfo with cache ([cc1a99f](https://www.github.com/sentriz/gonic/commit/cc1a99f03381a5afcebdbe95aaa42fb969f98b9f))
* **subsonic:** expose all of album "name"/"title"/"album" for browse by tag and browse by folder ([2df9052](https://www.github.com/sentriz/gonic/commit/2df9052bf9862258c81a69e362c23fa35e653831)), closes [#404](https://www.github.com/sentriz/gonic/issues/404)
* **subsonic:** expose track/album displayArtist/displayAlbumArtist ([0718aab](https://www.github.com/sentriz/gonic/commit/0718aabbacd52b7737dea606238ba64f65f2c2c6)), closes [#406](https://www.github.com/sentriz/gonic/issues/406)
* **subsonic:** support getAlbumList/getAlbumList2 `type=highest` ([a30ee3d](https://www.github.com/sentriz/gonic/commit/a30ee3d7f91ebf24f427e7f36b6b2830299935f9)), closes [#404](https://www.github.com/sentriz/gonic/issues/404)


### Bug Fixes

* add track count to /debug/vars metrics endpoint ([69c02e8](https://www.github.com/sentriz/gonic/commit/69c02e8352c10276697184d3c24d0f3253ec8c4d)), closes [#392](https://www.github.com/sentriz/gonic/issues/392)
* **contrib:** update config example ([d03d2dc](https://www.github.com/sentriz/gonic/commit/d03d2dc760c6c14b6f79efeca8b0111ead8912af))
* don't panic when scan on start fails ([37e826e](https://www.github.com/sentriz/gonic/commit/37e826e02b2cdcfc2d60bc860767f7dbd89a1dcd)), closes [#399](https://www.github.com/sentriz/gonic/issues/399)
* **metrics:** have a distinction between folders and albums ([cae3725](https://www.github.com/sentriz/gonic/commit/cae37255d68cbf493aba0bbb35dae38ca18fc4b6)), closes [#396](https://www.github.com/sentriz/gonic/issues/396)
* **scanner:** make sure we roll back invalid parents ([ddb686b](https://www.github.com/sentriz/gonic/commit/ddb686bddc5d040912812777829e6283d15c5343)), closes [#402](https://www.github.com/sentriz/gonic/issues/402)
* store and scrobble with real album artist info string ([fe0567a](https://www.github.com/sentriz/gonic/commit/fe0567a995dc40daeffa0460f0272b6e3af783a8))
* **subsonic:** don't return concatenated genres strings for song/trackchilds ([f18151b](https://www.github.com/sentriz/gonic/commit/f18151b75573e6f024529ec4a134ef213dd86a78))
* **subsonic:** songCount and albumCount in genre objects is required ([#390](https://www.github.com/sentriz/gonic/issues/390)) ([b17e76e](https://www.github.com/sentriz/gonic/commit/b17e76ea730e213d99a164c8ddff2c4b951f7f1f))
* use conf cache-path instead of XDG_CACHE_HOME for jukebox socket ([9818523](https://www.github.com/sentriz/gonic/commit/981852317572a8c6ab357e9ce81523780801d3fe)), closes [#391](https://www.github.com/sentriz/gonic/issues/391)

## [0.16.0](https://www.github.com/sentriz/gonic/compare/v0.15.2...v0.16.0) (2023-10-09)


### ⚠ BREAKING CHANGES

* **build:** bump to go 1.21
* **subsonic:** drop support for guessed artist covers in filesystem

### Features

* add .wav to list of supported audio types ([#322](https://www.github.com/sentriz/gonic/issues/322)) ([ab07b87](https://www.github.com/sentriz/gonic/commit/ab07b876b8686d2e2c792f2d422dd227d6f9d94b))
* add option for /debug/vars endpoint to expose database and media stats ([2a7a455](https://www.github.com/sentriz/gonic/commit/2a7a455ce27c3f8007a6114f815ae8ae94648533)), closes [#372](https://www.github.com/sentriz/gonic/issues/372) [#150](https://www.github.com/sentriz/gonic/issues/150)
* add support for wavpack ([#380](https://www.github.com/sentriz/gonic/issues/380)) ([827baf2](https://www.github.com/sentriz/gonic/commit/827baf2036dfe581bfddba1c5b88510c13b8bba6))
* **admin:** sort transcode profile names ([ae5bc2e](https://www.github.com/sentriz/gonic/commit/ae5bc2e1494f983993be7a053c953ca2f8555fae)), closes [#288](https://www.github.com/sentriz/gonic/issues/288)
* **admin:** support application/x-mpegurl playlist uploads ([6aa4c42](https://www.github.com/sentriz/gonic/commit/6aa4c42ce556f5a503bd9d5a4d1ee957c167dafa)), closes [#282](https://www.github.com/sentriz/gonic/issues/282)
* **admin:** update stylesheet ([222256c](https://www.github.com/sentriz/gonic/commit/222256cccbeb791168070ba5fe04a3bc1632cb94))
* allow multi valued tag modes to be configurable ([8f6610f](https://www.github.com/sentriz/gonic/commit/8f6610ff860ad18d56545b903d9edb6a1254ddec))
* **ci:** add a bunch more linters ([e3dd812](https://www.github.com/sentriz/gonic/commit/e3dd812b6c13c5f74690a868d0a5c37f30e33053))
* **ci:** update checkout and setup-go actions ([#326](https://www.github.com/sentriz/gonic/issues/326)) ([6144ac7](https://www.github.com/sentriz/gonic/commit/6144ac7979e03d704f8f721ba378eb612538d284))
* **ci:** update golangci-lint and action ([#325](https://www.github.com/sentriz/gonic/issues/325)) ([85eeb86](https://www.github.com/sentriz/gonic/commit/85eeb860bf1f97091fbd101b3bc827ff6502ccdb))
* **contrib:** improve system related contrib files ([ac74b35](https://www.github.com/sentriz/gonic/commit/ac74b354653b7353466eda3d81ca519bfa9f816e))
* enable profile-guided optimization ([e842b89](https://www.github.com/sentriz/gonic/commit/e842b896ec5acde6acdefbf3ec66610bd6cf7e22))
* **lastfm:** add user get loved tracks method ([9026c9e](https://www.github.com/sentriz/gonic/commit/9026c9e2c0463c467958b7032e4e6f7a889c3c76))
* **podcast:** parse podcast episode descriptions from HTML to plain text ([#351](https://www.github.com/sentriz/gonic/issues/351)) ([7d2c4fb](https://www.github.com/sentriz/gonic/commit/7d2c4fbb5c4cec3e7bcb4852214c17999bf320bb))
* **scanner:** add a new option for excluding paths based on a regexp ([1d38776](https://www.github.com/sentriz/gonic/commit/1d3877668f9bf925cae7c548b5d8e683d9676af2))
* **scanner:** support more cover types ([906164a](https://www.github.com/sentriz/gonic/commit/906164a5de34047444efe75b52b014737c111bc4))
* **scanner:** support non lowercase extensions like .Mp3 ([d83fe56](https://www.github.com/sentriz/gonic/commit/d83fe560d3df980c0bf99d03e026af9bce30e0c7))
* store and use m3u files on filesystem for playlists ([7dc9575](https://www.github.com/sentriz/gonic/commit/7dc9575e52538cf4fd39715193b5c760e1cc477b)), closes [#306](https://www.github.com/sentriz/gonic/issues/306) [#307](https://www.github.com/sentriz/gonic/issues/307) [#66](https://www.github.com/sentriz/gonic/issues/66)
* **subsonic:** add getOpenSubsonicExtensions endpoint and openSubsonic response key ([2caee44](https://www.github.com/sentriz/gonic/commit/2caee441ca49e7b2ca148f5b698365085db0cfc8))
* **subsonic:** add support for multi-valued album artist tags ([3ac7782](https://www.github.com/sentriz/gonic/commit/3ac77823c3bdc2e2be543f18e7b78295a7eb2fb8))
* **subsonic:** add support for podcast episodes in both playlists and play queues ([aecee3d](https://www.github.com/sentriz/gonic/commit/aecee3d2d859ca9e754db9f65bf031290d0ce4a5))
* **subsonic:** cache and use lastfm responses for covers, bios, top songs ([c374577](https://www.github.com/sentriz/gonic/commit/c374577328c17b6c3c7b6edbf901585e1f6644ee))
* **subsonic:** change frequent album list to use total time played per album instead of play count. ([#331](https://www.github.com/sentriz/gonic/issues/331)) ([7982ffc](https://www.github.com/sentriz/gonic/commit/7982ffc0b46bc3e14eb753cfb3791e183a75834b))
* **subsonic:** drop support for guessed artist covers in filesystem ([657fb22](https://www.github.com/sentriz/gonic/commit/657fb221db002ffebbf2a8d603566191acb40d17))
* **subsonic:** expose all album genres in a list of subsonic api ([749233d](https://www.github.com/sentriz/gonic/commit/749233db4effbdc7c3288f7cb8052784379be44f))
* **subsonic:** fetch artist images from lastfm opengraph ([4757495](https://www.github.com/sentriz/gonic/commit/475749534f784eb803eec9cb83c72d49f30d1994)), closes [#295](https://www.github.com/sentriz/gonic/issues/295)
* **subsonic:** gracefully handle missing podcast episode paths when returning playlists ([d5f8e23](https://www.github.com/sentriz/gonic/commit/d5f8e23a89cf1677e6ced124df4c0e07cdb7d114))
* **subsonic:** improve search2 and search3 when there are multiple words searched on. ([#335](https://www.github.com/sentriz/gonic/issues/335)) ([cbab68b](https://www.github.com/sentriz/gonic/commit/cbab68b05700aeb8b1fc2192ae5263ead248baf4))
* **subsonic:** make it easier to add more tag reading backends ([8382f61](https://www.github.com/sentriz/gonic/commit/8382f6123c2ac39d12b266730d76301faaf168f4))
* **subsonic:** order results from getStarred reverse chronologically based on star date ([b3c863c](https://www.github.com/sentriz/gonic/commit/b3c863c386adb830cdb45f45644f4779a4b73e86))
* **subsonic:** return artist cover ids for similar artists response ([c15349f](https://www.github.com/sentriz/gonic/commit/c15349f79667e71138c6088f52f8efbd9966c33a))
* **subsonic:** scrobble to different scrobble backends in parallel ([1ea2402](https://www.github.com/sentriz/gonic/commit/1ea240255927986b90a39c9ea67736f1b6808441))
* **subsonic:** support timeOffset in stream.view ([#384](https://www.github.com/sentriz/gonic/issues/384)) ([7eaf602](https://www.github.com/sentriz/gonic/commit/7eaf602e6990d6840f2fc5792599abb09bbef24b))
* **subsonic:** update track play stats on scrobble instead of stream ([e0b1603](https://www.github.com/sentriz/gonic/commit/e0b1603c00f7c24ce9b57085f5447bc0d7b016d7))
* **sunsonic:** expose type serverVersion in subsonic responses ([b8fceae](https://www.github.com/sentriz/gonic/commit/b8fceae3834d29f447fbb23eeb77f6216018e55a))
* **tags:** support multi valued tags like albumartists ([623d5c3](https://www.github.com/sentriz/gonic/commit/623d5c370906784b3ba8b21b89e435eafd41771e))
* **transcode:** add MP3 320 transcoding profile ([#363](https://www.github.com/sentriz/gonic/issues/363)) ([a644f0f](https://www.github.com/sentriz/gonic/commit/a644f0ff5c47c0f196c937dd76189de1e6099594))
* **transcode:** add opus 192 profile ([5dcc8c1](https://www.github.com/sentriz/gonic/commit/5dcc8c18a1f0cedf1ce6772aafe6dbdec1ded03c))
* **transcode:** lock the destination transcode cache path ([c9a2d2f](https://www.github.com/sentriz/gonic/commit/c9a2d2f9ce3fd7b9c724d8d2079abf43c2aa339b))


### Bug Fixes

* **admin:** continue on track match error ([16e6046](https://www.github.com/sentriz/gonic/commit/16e6046e85b99ebadc08f07b75404d7e67f6b77f))
* **podcasts:** make sure we use safeFilename for podcast episodes too ([#339](https://www.github.com/sentriz/gonic/issues/339)) ([5fb9c49](https://www.github.com/sentriz/gonic/commit/5fb9c49ed2810a1b63fce8930320eb7de0356c33))
* **podcasts:** remove query parameters from URL when getting the extension ([19be6f0](https://www.github.com/sentriz/gonic/commit/19be6f0881ed6c0c3311793c55dbbbe0a5571f51))
* **scanner:** fix watcher panic ([78d0c52](https://www.github.com/sentriz/gonic/commit/78d0c52d2240671a1e529339727bd013b7feb5c6))
* **scanner:** remove redundant mod time look up ([e0a8c18](https://www.github.com/sentriz/gonic/commit/e0a8c18b8df9b148053ed5dbe535f76c245e7df8)), closes [#293](https://www.github.com/sentriz/gonic/issues/293)
* **subsonic:** only return one bookmark entry per row in getBookmarks ([efe72fc](https://www.github.com/sentriz/gonic/commit/efe72fc447a23f48f0a49d92a7b3ed3c68d03722)), closes [#310](https://www.github.com/sentriz/gonic/issues/310)


### Miscellaneous Chores

* **build:** bump to go 1.21 ([658bae2](https://www.github.com/sentriz/gonic/commit/658bae2b43d170fa249ef2a0e953a494e0b2061a))

### [0.15.2](https://www.github.com/sentriz/gonic/compare/v0.15.1...v0.15.2) (2022-12-27)


### Bug Fixes

* **subsonic:** send valid content-type with http.ServeStream ([8dc58c7](https://www.github.com/sentriz/gonic/commit/8dc58c71a45b4bd8ec4224392a4d2ed3c36c24fa))

### [0.15.1](https://www.github.com/sentriz/gonic/compare/v0.15.0...v0.15.1) (2022-12-26)


### Features

* allow for custom music folder path alias ([7e097c9](https://www.github.com/sentriz/gonic/commit/7e097c9bdffa6f6cff57e5af6d9dea6965553c19)), closes [#259](https://www.github.com/sentriz/gonic/issues/259)
* **scrobble:** only send musicbrainz id if it's a valid uuid ([2bc3f31](https://www.github.com/sentriz/gonic/commit/2bc3f31554c9b6be9a9beef72b76e437d13131be))
* **server:** recover from panics ([df93286](https://www.github.com/sentriz/gonic/commit/df932864d812b6a83d1049c7a4820172c859ab22))
* **subsonic:** add stub lyrics.view ([0407a15](https://www.github.com/sentriz/gonic/commit/0407a1581f5d80dc66e849906518cb73a4dc3311)), closes [#274](https://www.github.com/sentriz/gonic/issues/274)


### Bug Fixes

* **jukebox:** gracefully handle case of no audio in feed item ([b47c880](https://www.github.com/sentriz/gonic/commit/b47c880ea5ffd357878b7617c3493d7b1f48f4cf)), closes [#269](https://www.github.com/sentriz/gonic/issues/269)
* **jukebox:** use a tmp dir instead of file for mpv sock ([4280700](https://www.github.com/sentriz/gonic/commit/42807006213871432b7ddfbb623ed6f82f4122c6)), closes [#266](https://www.github.com/sentriz/gonic/issues/266) [#265](https://www.github.com/sentriz/gonic/issues/265)
* **subsonic:** update music folder id in bounds check ([c6ddee8](https://www.github.com/sentriz/gonic/commit/c6ddee8f7e7f246c4384b05384057c24231937a1)), closes [#271](https://www.github.com/sentriz/gonic/issues/271)
* **transcode:** don't leave half transcode cache files lying around ([ce31310](https://www.github.com/sentriz/gonic/commit/ce31310571a1b1c3007202d507bec5ef1a9fad99)), closes [#270](https://www.github.com/sentriz/gonic/issues/270)

## [0.15.0](https://www.github.com/sentriz/gonic/compare/v0.14.0...v0.15.0) (2022-11-17)


### ⚠ BREAKING CHANGES

* upgrade deps and require go 1.19
* **podcast:** make podcasts global not per user, to match spec

### Features

* add a ping endpoint that doesn't create a session ([731a696](https://www.github.com/sentriz/gonic/commit/731a696bd795bbec668e4685ed1cb2271044c8aa))
* add CreatedAt to albums ([#159](https://www.github.com/sentriz/gonic/issues/159)) ([848d85d](https://www.github.com/sentriz/gonic/commit/848d85d26a4c2a6e83cd01c21e63080fdbb27cd8))
* add multi folder support ([40cd031](https://www.github.com/sentriz/gonic/commit/40cd031b05c71d930b5d92ed6ebbbf676f5e219e)), closes [#50](https://www.github.com/sentriz/gonic/issues/50)
* **countrw:** add countrw package ([5155dee](https://www.github.com/sentriz/gonic/commit/5155dee2e82972ef50adfdbe2298b1126dcd994d))
* **jukebox:** allow users to pass custom arguments to mpv ([428fdda](https://www.github.com/sentriz/gonic/commit/428fddad1bad0dc7091528794622fc2d2dc7c1dc)), closes [#125](https://www.github.com/sentriz/gonic/issues/125) [#164](https://www.github.com/sentriz/gonic/issues/164)
* **jukebox:** use mpv over ipc as a player backend ([e1488b0](https://www.github.com/sentriz/gonic/commit/e1488b0d183a3244eba89220ad960865b94087e5))
* **lastfm:** scrobble with duration ([7d0d036](https://www.github.com/sentriz/gonic/commit/7d0d036f0bc0f5bd4429f510ca636daa3934766b))
* log all folders while scanning ([b2388e6](https://www.github.com/sentriz/gonic/commit/b2388e6d851c2192bda14eb7771c83ce75f493f9))
* **mockfs:** add DumpDB method ([b0d5861](https://www.github.com/sentriz/gonic/commit/b0d5861d10a5472d4ac07a5e27331ec492be69b6))
* **podcasts:** add an option to purge old episodes ([85cb0fe](https://www.github.com/sentriz/gonic/commit/85cb0feb5a11b753bc7936e040e00e95e6601b47))
* render local artist images for getArtistInfo2 ([cb6b33a](https://www.github.com/sentriz/gonic/commit/cb6b33a9fb69589bf73e33773c6f16bf073ce865))
* render local artist images with no foreign key ([a74b5a2](https://www.github.com/sentriz/gonic/commit/a74b5a261c5d47c1a24942ecd4ddd98666755ad4))
* **scanner:** add fuzzing test ([f7f4b8b](https://www.github.com/sentriz/gonic/commit/f7f4b8b2cc3fffaa85a63fceba1bdc7cd79c9044))
* **scanner:** add option to use fsnotify based scan watcher ([#232](https://www.github.com/sentriz/gonic/issues/232)) ([ea28ff1](https://www.github.com/sentriz/gonic/commit/ea28ff1df3f0ea30c53bc79c2e9980ea7ad7206b))
* **scanner:** added option to scan at startup ([f6c9550](https://www.github.com/sentriz/gonic/commit/f6c95503c714dce11bddc4db632028ab09992093)), closes [#251](https://www.github.com/sentriz/gonic/issues/251)
* **server:** support TLS ([59c4047](https://www.github.com/sentriz/gonic/commit/59c404749fa71416e29facac4ec523acd65a0f01))
* **subsonic:** add avatar support ([5e66261](https://www.github.com/sentriz/gonic/commit/5e66261f0ccd63e6ceda46dc908661a748c16325)), closes [#228](https://www.github.com/sentriz/gonic/issues/228)
* **subsonic:** add detailed logging about requested audio ([dc4d9e4](https://www.github.com/sentriz/gonic/commit/dc4d9e4e96c905f6edcfcdddae0a16214b3b054d)), closes [#212](https://www.github.com/sentriz/gonic/issues/212)
* **subsonic:** add getNewestPodcasts ([f6687df](https://www.github.com/sentriz/gonic/commit/f6687df3f3f0d94a2db661b9d4b276175d951d68))
* **subsonic:** add internet radio support ([7ab378a](https://www.github.com/sentriz/gonic/commit/7ab378accbadf2f25478ae37e231aacca881f7b7))
* **subsonic:** add support for track/album/artist ratings/stars ([e8759cb](https://www.github.com/sentriz/gonic/commit/e8759cb6c11cd61a2a8ca3892fc905c4a9c4b167))
* **subsonic:** add year and genre fields to track-by-folder response ([53a4247](https://www.github.com/sentriz/gonic/commit/53a4247dfd18d0783316d6a38126eca3f9df8af9)), closes [#223](https://www.github.com/sentriz/gonic/issues/223)
* **subsonic:** implement getSimilarSongs.view ([e1cfed7](https://www.github.com/sentriz/gonic/commit/e1cfed7965ed43c91689fd5949dab55fa77a983d)), closes [#195](https://www.github.com/sentriz/gonic/issues/195)
* **subsonic:** implement getSimilarSongs2.view ([92febcf](https://www.github.com/sentriz/gonic/commit/92febcffe6bbaff487b6869fbd3467003c987bed)), closes [#195](https://www.github.com/sentriz/gonic/issues/195)
* **subsonic:** implement getTopSongs.view ([39b3ae5](https://www.github.com/sentriz/gonic/commit/39b3ae5ecb2ddb8c733beb99c80b68356d203be2)), closes [#195](https://www.github.com/sentriz/gonic/issues/195)
* **subsonic:** improve getArtistInfo2.view similar artist results ([#203](https://www.github.com/sentriz/gonic/issues/203)) ([55c0920](https://www.github.com/sentriz/gonic/commit/55c09209b6ebdc0ecd7ca17d5b173a8db0cb23b1))
* **subsonic:** log error responses ([2440e69](https://www.github.com/sentriz/gonic/commit/2440e696892b38d2cc255373f700c2449a98fef2))
* **subsonic:** make the v param optional ([50e2818](https://www.github.com/sentriz/gonic/commit/50e2818cc78aad1c5ecdb388dd0ebb5e16f0ae26))
* **subsonic:** return transcoded mime and transcoded suffix in subsonic responses ([6e6404a](https://www.github.com/sentriz/gonic/commit/6e6404af73357b4abcf0d45d2e8114f4404d1b5c))
* **subsonic:** skip transcoding if request bitrate is the same as track bitrate ([f41dd08](https://www.github.com/sentriz/gonic/commit/f41dd0818ba95e27fbad376438585a1057f60382)), closes [#241](https://www.github.com/sentriz/gonic/issues/241)
* **subsonic:** sort artist album list ([e56f64a](https://www.github.com/sentriz/gonic/commit/e56f64a75877efe15f96414c5dc58a33b03cb9ce)), closes [#197](https://www.github.com/sentriz/gonic/issues/197)
* **subsonic:** support dsub edgecase for queries by decade ([03df207](https://www.github.com/sentriz/gonic/commit/03df207e638122446a9b24facfd0b893ddd9e0e8))
* **subsonic:** support public playlists ([1647eaa](https://www.github.com/sentriz/gonic/commit/1647eaac4585cca7a244036f9c242a5602706b83))
* **subsonic:** update play stats when scrobbling ([1ab47d6](https://www.github.com/sentriz/gonic/commit/1ab47d6fbee83f2dd00256bf5cd9ad33c2448202)), closes [#207](https://www.github.com/sentriz/gonic/issues/207)
* **transcode:** add a generic transcoding package for encoding/decoding/caching ([165904c](https://www.github.com/sentriz/gonic/commit/165904c2bb2857aacc9053759ff707d64389a3bb))
* **transcode:** add opus 128 kbps profiles ([bb83426](https://www.github.com/sentriz/gonic/commit/bb83426816a3f5fd45f14a5114e6843923598b21)), closes [#241](https://www.github.com/sentriz/gonic/issues/241)
* **ui:** add a link to wiki in transcode profile section ([3348ca6](https://www.github.com/sentriz/gonic/commit/3348ca6b5bf0d054c8be38e860acdecc6e040b1c)), closes [#254](https://www.github.com/sentriz/gonic/issues/254)
* **ui:** show when a scan is in progress ([7fbe7c0](https://www.github.com/sentriz/gonic/commit/7fbe7c0994356e5adaaa160e51ac4ae051ea027b))
* use album create time for home ui and album listings ([14a2668](https://www.github.com/sentriz/gonic/commit/14a266842600fae27a134590d69542dc5d0d2cfc)), closes [#182](https://www.github.com/sentriz/gonic/issues/182) [#135](https://www.github.com/sentriz/gonic/issues/135)


### Bug Fixes

* add stub getStarred views to shut up refix ([27ac8e1](https://www.github.com/sentriz/gonic/commit/27ac8e1d25d9b58a8c71b9f7318a6b398f4a5865))
* **ci:** set golangci-lint timeout ([48c34fd](https://www.github.com/sentriz/gonic/commit/48c34fdffc1c9bc47ce57d26b433dbbd775831a6))
* **docs:** add GONIC_HTTP_LOG to setting table ([a11d6ab](https://www.github.com/sentriz/gonic/commit/a11d6ab92d3661d2311a1567bf8fffa07dd1eee6))
* don't send listenbrainz playing_now and submitted_at at the same time ([b07b9a8](https://www.github.com/sentriz/gonic/commit/b07b9a8be610a932d6c66839f020456ff136d2f6)), closes [#168](https://www.github.com/sentriz/gonic/issues/168)
* **lastfm:** make a better guess at callback protocol when incoming connection is TLS ([4658d07](https://www.github.com/sentriz/gonic/commit/4658d0727323fdf8107f94c7b0a61c419e6504f6)), closes [#213](https://www.github.com/sentriz/gonic/issues/213)
* **listenbrainz:** set json header ([e883de8](https://www.github.com/sentriz/gonic/commit/e883de8c957a23d14103e547c7ddbbab161a43db))
* **listenbrainz:** submit track recording ID instead of track ID ([8ee357b](https://www.github.com/sentriz/gonic/commit/8ee357b0217eeeebbee954111e17e4d29ac09c91)), closes [#240](https://www.github.com/sentriz/gonic/issues/240)
* make sure open cover and audio files are closed after use ([1d1ab11](https://www.github.com/sentriz/gonic/commit/1d1ab116cd331fb5dbce50051f61be42e771ff80))
* **podcast:** add error case for when DownloadEpisode is called via API and podcast is already downloaded ([611bc96](https://www.github.com/sentriz/gonic/commit/611bc96e29abd69e322b0a33705c164b1577dd99)), closes [#213](https://www.github.com/sentriz/gonic/issues/213)
* **podcast:** add user agent to avoid 403s with some remotes ([0f80ae2](https://www.github.com/sentriz/gonic/commit/0f80ae2655509ddcc044c4e113f5ec1eaed77050))
* render artistId in track types ([7ec6440](https://www.github.com/sentriz/gonic/commit/7ec6440ed2c95b0f38b8089c17dcd23a2d26bf23)), closes [#170](https://www.github.com/sentriz/gonic/issues/170)
* **scanner:** better detect years given extraneous year tags ([a9d3933](https://www.github.com/sentriz/gonic/commit/a9d393350a0286d77d8eb1c68d46e5eb4c2e5cc8))
* **scanner:** fix linting Ctim.Sec/Ctim.Nsec on 32 bit systems ([b280e8d](https://www.github.com/sentriz/gonic/commit/b280e8d256d28cfff6d135d8bf5eadc576e34d45))
* **scanner:** fix records with album name same as artist ([fdbb282](https://www.github.com/sentriz/gonic/commit/fdbb28209b3e155825fa5380774102e2f119e22e)), closes [#230](https://www.github.com/sentriz/gonic/issues/230)
* **scanner:** make sure we have an album artist before populating track ([01747c8](https://www.github.com/sentriz/gonic/commit/01747c89400decedaaa0f801bb9aeb8a7f6e75f5)), closes [#209](https://www.github.com/sentriz/gonic/issues/209)
* **scanner:** respect "is full" setting ([f2143e3](https://www.github.com/sentriz/gonic/commit/f2143e32ef42ae25875db62a2337a4770e095798))
* set ON DELETE SET NULL to artists.guessed_folder_id removing folders ([24d64e2](https://www.github.com/sentriz/gonic/commit/24d64e2125995bbe446fcadc449cc0914a70202c))
* show artist album count when searching by tags ([0c79044](https://www.github.com/sentriz/gonic/commit/0c790442f4fc0c53dd0c71c05b66c600db883b9a))
* show artist covers (raw url in artist info, cover id elsewhere) via scanned guessed artist folder ([c0ebd26](https://www.github.com/sentriz/gonic/commit/c0ebd2642206f7dba81f136cd9d576ded75bb14e)), closes [#180](https://www.github.com/sentriz/gonic/issues/180) [#179](https://www.github.com/sentriz/gonic/issues/179)
* **subsonic:** change order of fromYear toYear query ([d7655cb](https://www.github.com/sentriz/gonic/commit/d7655cb9d167222446c32a21eb4951b75e12857d)), closes [#208](https://www.github.com/sentriz/gonic/issues/208)
* **subsonic:** correct album orderding in getAlbumList, add starred request type in getAlbumList ([692ec68](https://www.github.com/sentriz/gonic/commit/692ec68282805e2dc3cf4a74ac7a44a249fe3695))
* **subsonic:** return an error when no tracks provided in savePlayQueue ([d47d5e1](https://www.github.com/sentriz/gonic/commit/d47d5e17e91d1775e3c6f16d900ba3b565401393))
* **subsonic:** return song artist ID, album and song genres from search3 ([1a1f39f](https://www.github.com/sentriz/gonic/commit/1a1f39f4e8e5553f32499b1461d227a93820e70f)), closes [#229](https://www.github.com/sentriz/gonic/issues/229)
* **subsonic:** route settings.view -> admin home ([f9133aa](https://www.github.com/sentriz/gonic/commit/f9133aac91e5f18473dc461a6f2ffd0417967465))


### Code Refactoring

* **podcast:** make podcasts global not per user, to match spec ([182c96e](https://www.github.com/sentriz/gonic/commit/182c96e9669369d862787f46b541b5090cd64887))


### Miscellaneous Chores

* upgrade deps and require go 1.19 ([385a980](https://www.github.com/sentriz/gonic/commit/385a980b715e7259f896198b4ad4624c40e1e9dd))

## [0.14.0](https://www.github.com/sentriz/gonic/compare/v0.13.1...v0.14.0) (2021-10-03)


### Features

* **ci:** add debug build workflow ([2780dba](https://www.github.com/sentriz/gonic/commit/2780dba534b673b1a496d44c9fcc3007fd0f2e62))
* **ci:** pin golangci-lint version ([8f7131e](https://www.github.com/sentriz/gonic/commit/8f7131e25b9ea4207cdb9091ccbae26b5118cdac))
* **ci:** test before release please, and only run extra tests on develop and pull request ([cd5771f](https://www.github.com/sentriz/gonic/commit/cd5771f88635b95955c7d2aea72379411142b777))
* **ci:** use GITHUB_TOKEN for release please ([608504b](https://www.github.com/sentriz/gonic/commit/608504bedc88ec02cef34849cb42fb476dd63e1c))
* create cache directory on startup ([f3bc3ae](https://www.github.com/sentriz/gonic/commit/f3bc3ae78990948e75d0b9757c399aad4e5c3b6b)), closes [#127](https://www.github.com/sentriz/gonic/issues/127)
* **encode:** add hi-gain RG and upsampling support ([616b152](https://www.github.com/sentriz/gonic/commit/616b152fede7d56b77b8ea96bc2b86226d690f93))
* **encode:** add mime-type headers to cache handlers ([4109b5b](https://www.github.com/sentriz/gonic/commit/4109b5b66cbb53e9255fcd216195f8e1a773e48d))
* **encode:** use "true" (unconstrained) VBR for Opus profiles ([b9f8ea7](https://www.github.com/sentriz/gonic/commit/b9f8ea704876eb033986e7e586f16c93e2864df2))
* **jukebox:** reduce complexity and update dependencies ([#154](https://www.github.com/sentriz/gonic/issues/154)) ([3938136](https://www.github.com/sentriz/gonic/commit/393813665abb614fa2e2f57cdd575c4dd083b4b5))
* support filter by genre in browse by folder mode ([b56f00e](https://www.github.com/sentriz/gonic/commit/b56f00e9ace62fc3d60b21eef7e638b1ec5007d7))
* support filter by year in browse by folder mode ([6e2d4f7](https://www.github.com/sentriz/gonic/commit/6e2d4f73c53ab908b5933cfbbc1ffc97584e0a08))
* Support WMA files, including those with embedded album art ([#143](https://www.github.com/sentriz/gonic/issues/143)) ([7100b2b](https://www.github.com/sentriz/gonic/commit/7100b2b241ab5c199aaa17b2631b85b065b383e1))


### Bug Fixes

* **build:** add zlib ([ccc0e3c](https://www.github.com/sentriz/gonic/commit/ccc0e3c58d9fb1975bc0bdcf4f87829e9f705b74))
* **ci:** remove deprecated linters ([3382af7](https://www.github.com/sentriz/gonic/commit/3382af72f19eead97b601eee847fd60b6c50ca34))
* **ci:** trim short hash ([6f26974](https://www.github.com/sentriz/gonic/commit/6f269745a5f678b256b4a715ba236a2b847e4de9))
* **docs:** update ubuntu / systemd service instructions ([ef6dd6c](https://www.github.com/sentriz/gonic/commit/ef6dd6c82a638dcd8aa3254839e2f53580a4ef46)), closes [#126](https://www.github.com/sentriz/gonic/issues/126)
* **encode:** Strip EBU R128 gain tags when using forced-RG transcoding ([#145](https://www.github.com/sentriz/gonic/issues/145)) ([5444d40](https://www.github.com/sentriz/gonic/commit/5444d40018c6f8051fc8d03ef46bd66737dfe1f4))
* return early before type switch in ServeStream ([212a133](https://www.github.com/sentriz/gonic/commit/212a13395d288486f9baa57c2da9bef2d9b6324d)), closes [#152](https://www.github.com/sentriz/gonic/issues/152)
* **scanner:** refactor a bit and fix the issue of repeatedly adding and removing tracks 😎 ([93608d0](https://www.github.com/sentriz/gonic/commit/93608d04b49ebfde3020752802fd665ccfe807bb)), closes [#26](https://www.github.com/sentriz/gonic/issues/26) [#63](https://www.github.com/sentriz/gonic/issues/63)
* **scanner:** spawn interval scans in a goroutine ([c0ca6aa](https://www.github.com/sentriz/gonic/commit/c0ca6aaf0337a23b3f1d2a867884afe89fd4a281)), closes [#63](https://www.github.com/sentriz/gonic/issues/63)
* **scanner:** update changed cover files when scanning ([f50817a](https://www.github.com/sentriz/gonic/commit/f50817a3dcdaf752ac4c9a20c078428846dc2bde)), closes [#158](https://www.github.com/sentriz/gonic/issues/158)
* show "gonic" not version in --help ([3cf3bda](https://www.github.com/sentriz/gonic/commit/3cf3bdafd890ea25247f2bf9af14e775d8d1d148))
* trim newlines when rendering flag values ([4637cf7](https://www.github.com/sentriz/gonic/commit/4637cf70c16d9c4ea30c9604ca79704ec0e3f0c4))

### [0.13.1](https://www.github.com/sentriz/gonic/compare/v0.13.0...v0.13.1) (2021-05-08)


### Bug Fixes

* **docker:** bump alpine / go ([1f941b2](https://www.github.com/sentriz/gonic/commit/1f941b2085815d8aa0bf7ad7f3e44efba20295e8))

## [0.13.0](https://www.github.com/sentriz/gonic/compare/v0.12.3...v0.13.0) (2021-05-08)


### ⚠ BREAKING CHANGES

* **subsonic:** don't return gonic version from responses
* bump to go1.16 and embed version

### Features

* bump to go1.16 and embed version ([6f15589](https://www.github.com/sentriz/gonic/commit/6f15589c0889893b7beda85a81d49878401566f0))
* **ci:** arm builds, push multiple registries ([0622672](https://www.github.com/sentriz/gonic/commit/06226724b718883cff9e9150e60e2eeacc2e0a1c))
* **ci:** use ghcr and auto release ([c2c7eb2](https://www.github.com/sentriz/gonic/commit/c2c7eb249f77eebabc910c70357249a3017523ef))
* **subsonic:** don't return gonic version from responses ([58624f0](https://www.github.com/sentriz/gonic/commit/58624f07dc81c36fda79827cc41ac57e89e18b37))


### Bug Fixes

* **ci:** only test on go1.16 ([e9743f0](https://www.github.com/sentriz/gonic/commit/e9743f0cb0e96e9b4b434141e890a0fa16ce3f18))
* don't clutter db close in main ([e6b7691](https://www.github.com/sentriz/gonic/commit/e6b76915daa2bbd6f259f2b019cde9130c62e326))
* trim newline from version ([a565008](https://www.github.com/sentriz/gonic/commit/a5650084d7969a37765d291a6554984e4ae4e2d9))
