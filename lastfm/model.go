package lastfm

import "encoding/xml"

type (
	LastFM struct {
		XMLName        xml.Name       `xml:"lfm"`
		Status         string         `xml:"status,attr"`
		Session        Session        `xml:"session"`
		Error          Error          `xml:"error"`
		Artist         Artist         `xml:"artist"`
		Album          Album          `xml:"album"`
		TopTracks      TopTracks      `xml:"toptracks"`
		SimilarTracks  SimilarTracks  `xml:"similartracks"`
		SimilarArtists SimilarArtists `xml:"similarartists"`
		LovedTracks    LovedTracks    `xml:"lovedtracks"`
		User           User           `xml:"user"`
	}

	Session struct {
		Name       string `xml:"name"`
		Key        string `xml:"key"`
		Subscriber uint   `xml:"subscriber"`
	}

	Error struct {
		Code  uint   `xml:"code,attr"`
		Value string `xml:",chardata"`
	}

	SimilarArtist struct {
		XMLName    xml.Name `xml:"artist"`
		Name       string   `xml:"name"`
		MBID       string   `xml:"mbid"`
		URL        string   `xml:"url"`
		Image      []Image  `xml:"image"`
		Streamable string   `xml:"streamable"`
	}

	Image struct {
		Text string `xml:",chardata"`
		Size string `xml:"size,attr"`
	}

	Artist struct {
		XMLName    xml.Name `xml:"artist"`
		Name       string   `xml:"name"`
		MBID       string   `xml:"mbid"`
		URL        string   `xml:"url"`
		Image      []Image  `xml:"image"`
		Streamable string   `xml:"streamable"`
		Stats      struct {
			Listeners string `xml:"listeners"`
			Playcount string `xml:"playcount"`
		} `xml:"stats"`
		Similar struct {
			Artists []Artist `xml:"artist"`
		} `xml:"similar"`
		Tags struct {
			Tag []ArtistTag `xml:"tag"`
		} `xml:"tags"`
		Bio ArtistBio `xml:"bio"`
	}

	Album struct {
		XMLName xml.Name `xml:"album"`
		Name    string   `xml:"name"`
		Artist  string   `xml:"artist"`
		MBID    string   `xml:"mbid"`
		URL     string   `xml:"url"`
		Image   []struct {
			Text string `xml:",chardata"`
			Size string `xml:"size,attr"`
		} `xml:"image"`
		Listeners string `xml:"listeners"`
		Playcount string `xml:"playcount"`
		Tracks    struct {
			Text  string `xml:",chardata"`
			Track []struct {
				Text       string `xml:",chardata"`
				Rank       string `xml:"rank,attr"`
				Name       string `xml:"name"`
				URL        string `xml:"url"`
				Duration   string `xml:"duration"`
				Streamable struct {
					Text      string `xml:",chardata"`
					Fulltrack string `xml:"fulltrack,attr"`
				} `xml:"streamable"`
				Artist struct {
					Text string `xml:",chardata"`
					Name string `xml:"name"`
					Mbid string `xml:"mbid"`
					URL  string `xml:"url"`
				} `xml:"artist"`
			} `xml:"track"`
		} `xml:"tracks"`
		Tags struct {
			Text string `xml:",chardata"`
			Tag  []struct {
				Text string `xml:",chardata"`
				Name string `xml:"name"`
				URL  string `xml:"url"`
			} `xml:"tag"`
		} `xml:"tags"`
		Wiki struct {
			Text      string `xml:",chardata"`
			Published string `xml:"published"`
			Summary   string `xml:"summary"`
			Content   string `xml:"content"`
		} `xml:"wiki"`
	}

	ArtistTag struct {
		Name string `xml:"name"`
		URL  string `xml:"url"`
	}

	ArtistBio struct {
		Published string `xml:"published"`
		Summary   string `xml:"summary"`
		Content   string `xml:"content"`
	}

	TopTracks struct {
		XMLName xml.Name `xml:"toptracks"`
		Artist  string   `xml:"artist,attr"`
		Tracks  []Track  `xml:"track"`
	}

	SimilarTracks struct {
		XMLName xml.Name `xml:"similartracks"`
		Artist  string   `xml:"artist,attr"`
		Track   string   `xml:"track,attr"`
		Tracks  []Track  `xml:"track"`
	}

	SimilarArtists struct {
		XMLName xml.Name `xml:"similarartists"`
		Artist  string   `xml:"artist,attr"`
		Artists []Artist `xml:"artist"`
	}

	Track struct {
		Rank      int     `xml:"rank,attr"`
		Tracks    []Track `xml:"track"`
		Name      string  `xml:"name"`
		MBID      string  `xml:"mbid"`
		PlayCount int     `xml:"playcount"`
		Listeners int     `xml:"listeners"`
		URL       string  `xml:"url"`
		Image     []Image `xml:"image"`
	}

	LovedTracks struct {
		XMLName xml.Name `xml:"lovedtracks"`
		Tracks  []struct {
			Track
			Date struct {
				Text string `xml:",chardata"`
				UTS  string `xml:"uts,attr"`
			} `xml:"date"`
			Artist Artist `xml:"artist"`
		} `xml:"track"`
	}

	User struct {
		Text     string `xml:",chardata"`
		Name     string `xml:"name"`
		Realname string `xml:"realname"`
		Image    []struct {
			Text string `xml:",chardata"`
			Size string `xml:"size,attr"`
		} `xml:"image"`
		URL        string `xml:"url"`
		Country    string `xml:"country"`
		Age        string `xml:"age"`
		Gender     string `xml:"gender"`
		Subscriber string `xml:"subscriber"`
		Playcount  string `xml:"playcount"`
		Playlists  string `xml:"playlists"`
		Bootstrap  string `xml:"bootstrap"`
		Registered struct {
			Text     string `xml:",chardata"`
			Unixtime string `xml:"unixtime,attr"`
		} `xml:"registered"`
		Type        string `xml:"type"`
		ArtistCount string `xml:"artist_count"`
		AlbumCount  string `xml:"album_count"`
		TrackCount  string `xml:"track_count"`
	}
)
