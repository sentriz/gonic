package listenbrainz

// https://listenbrainz.readthedocs.io/en/latest/users/json.html#submission-json

type (
	Payload struct {
		ListenedAt    int            `json:"listened_at,omitempty"`
		TrackMetadata *TrackMetadata `json:"track_metadata"`
	}

	AdditionalInfo struct {
		TrackNumber   int    `json:"tracknumber,omitempty"`
		TrackMBID     string `json:"track_mbid,omitempty"`
		RecordingMBID string `json:"recording_mbid,omitempty"`
		Duration      int    `json:"duration,omitempty"`
	}

	TrackMetadata struct {
		AdditionalInfo *AdditionalInfo `json:"additional_info"`
		ArtistName     string          `json:"artist_name,omitempty"`
		TrackName      string          `json:"track_name,omitempty"`
		ReleaseName    string          `json:"release_name,omitempty"`
	}

	Scrobble struct {
		ListenType string     `json:"listen_type,omitempty"`
		Payload    []*Payload `json:"payload"`
	}
)
