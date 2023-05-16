package lastfm

import "encoding/xml"

type (
	LastFM struct {
		XMLName        xml.Name       `xml:"lfm"`
		Status         string         `xml:"status,attr"`
		Session        Session        `xml:"session"`
		Error          Error          `xml:"error"`
		Artist         Artist         `xml:"artist"`
		TopTracks      TopTracks      `xml:"toptracks"`
		SimilarTracks  SimilarTracks  `xml:"similartracks"`
		SimilarArtists SimilarArtists `xml:"similarartists"`
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
		XMLName xml.Name `xml:"artist"`
		Name    string   `xml:"name"`
		MBID    string   `xml:"mbid"`
		URL     string   `xml:"url"`
		Image   []struct {
			Text string `xml:",chardata"`
			Size string `xml:"size,attr"`
		} `xml:"image"`
		Streamable string `xml:"streamable"`
	}

	ArtistImage struct {
		Text string `xml:",chardata"`
		Size string `xml:"size,attr"`
	}

	Artist struct {
		XMLName    xml.Name      `xml:"artist"`
		Name       string        `xml:"name"`
		MBID       string        `xml:"mbid"`
		URL        string        `xml:"url"`
		Image      []ArtistImage `xml:"image"`
		Streamable string        `xml:"streamable"`
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
		Image     []struct {
			Text string `xml:",chardata"`
			Size string `xml:"size,attr"`
		} `xml:"image"`
	}
)
