package listenbrainz

type LastFM struct {
	Error   Error
}

type Error struct {
	Code  uint
	Value string
}
