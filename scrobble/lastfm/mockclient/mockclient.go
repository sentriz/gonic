package mockclient

import (
	"context"
	"crypto/tls"
	_ "embed"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func New(t testing.TB, handler http.HandlerFunc) *http.Client {
	server := httptest.NewTLSServer(handler)
	t.Cleanup(server.Close)

	return &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial(network, server.Listener.Addr().String())
			},
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, //nolint:gosec
			},
		},
	}
}

//go:embed artist_get_info_response.xml
var ArtistGetInfoResponse []byte

//go:embed artist_get_top_tracks_response.xml
var ArtistGetTopTracksResponse []byte

//go:embed artist_get_similar_response.xml
var ArtistGetSimilarResponse []byte

//go:embed track_get_similar_response.xml
var TrackGetSimilarResponse []byte

//go:embed get_session_response.xml
var GetSessionResponse []byte
