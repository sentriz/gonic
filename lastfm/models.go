package lastfm

import "encoding/xml"

type LastFM struct {
	XMLName xml.Name `xml:"lfm"`
	Status  string   `xml:"status,attr"`
	Session *Session `xml:"session"`
	Error   *Error   `xml:"error"`
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
