package lastfm

import (
	"encoding/xml"

	"go.senan.xyz/gonic/server/db"
)

type Scrobbler interface {
	Scrobble(*db.User, ScrobbleOptions) error
	Enabled(*db.User) bool
}

type LastFM struct {
	XMLName xml.Name `xml:"lfm"`
	Status  string   `xml:"status,attr"`
	Session Session  `xml:"session"`
	Error   Error    `xml:"error"`
	Artist  Artist   `xml:"artist"`
}

type Session struct {
	Name       string `xml:"name"`
	Key        string `xml:"key"`
	Subscriber uint   `xml:"subscriber"`
}

type Error struct {
	Code  uint   `xml:"code,attr"`
	Value string `xml:",chardata"`
}

type Artist struct {
	XMLName xml.Name `xml:"artist"`
	Name    string   `xml:"name"`
	MBID    string   `xml:"mbid"`
	URL     string   `xml:"url"`
	Image   []struct {
		Text string `xml:",chardata"`
		Size string `xml:"size,attr"`
	} `xml:"image"`
	Streamable string `xml:"streamable"`
	Stats      struct {
		Listeners string `xml:"listeners"`
		Plays     string `xml:"plays"`
	} `xml:"stats"`
	Similar struct {
		Artists []Artist `xml:"artist"`
	} `xml:"similar"`
	Tags struct {
		Tag []ArtistTag `xml:"tag"`
	} `xml:"tags"`
	Bio ArtistBio `xml:"bio"`
}

type ArtistTag struct {
	Name string `xml:"name"`
	URL  string `xml:"url"`
}

type ArtistBio struct {
	Published string `xml:"published"`
	Summary   string `xml:"summary"`
	Content   string `xml:"content"`
}

type ListenBrainzAdditionalInfo struct {
	TrackNumber int `json:"tracknumber"`
}

type ListenBrainzTrackMetadata struct {
	AdditionalInfo ListenBrainzAdditionalInfo `json:"additional_info"`
	ArtistName     string                     `json:"artist_name"`
	TrackName      string                     `json:"track_name"`
	ReleaseName    string                     `json:"release_name"`
}

type ListenBrainzPayload struct {
	ListenedAt    int                       `json:"listened_at"`
	TrackMetadata ListenBrainzTrackMetadata `json:"track_metadata"`
}

type ListenBrainzScrobble struct {
	ListenType string                `json:"listen_type"`
	Payload    []ListenBrainzPayload `json:"payload"`
}
