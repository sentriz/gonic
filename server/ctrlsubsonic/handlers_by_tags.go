package ctrlsubsonic

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/server/lastfm"
)

func (c *Controller) ServeGetArtists(r *http.Request) *spec.Response {
	var artists []*db.Artist
	c.DB.
		Select("*, count(sub.id) album_count").
		Joins("LEFT JOIN albums sub ON artists.id=sub.tag_artist_id").
		Group("artists.id").
		Order("artists.name COLLATE NOCASE").
		Find(&artists)
	// [a-z#] -> 27
	indexMap := make(map[string]*spec.Index, 27)
	resp := make([]*spec.Index, 0, 27)
	for _, artist := range artists {
		i := lowerUDecOrHash(artist.IndexName())
		index, ok := indexMap[i]
		if !ok {
			index = &spec.Index{
				Name:    i,
				Artists: []*spec.Artist{},
			}
			indexMap[i] = index
			resp = append(resp, index)
		}
		index.Artists = append(index.Artists,
			spec.NewArtistByTags(artist))
	}
	sub := spec.NewResponse()
	sub.Artists = &spec.Artists{
		List: resp,
	}
	return sub
}

func (c *Controller) ServeGetArtist(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	artist := &db.Artist{}
	c.DB.
		Preload("Albums").
		First(artist, id.Value)
	sub := spec.NewResponse()
	sub.Artist = spec.NewArtistByTags(artist)
	sub.Artist.Albums = make([]*spec.Album, len(artist.Albums))
	for i, album := range artist.Albums {
		sub.Artist.Albums[i] = spec.NewAlbumByTags(album, artist)
	}
	sub.Artist.AlbumCount = len(artist.Albums)
	return sub
}

func (c *Controller) ServeGetAlbum(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	album := &db.Album{}
	err = c.DB.
		Preload("TagArtist").
		Preload("TagGenre").
		Preload("Tracks", func(db *gorm.DB) *gorm.DB {
			return db.Order("tracks.tag_disc_number, tracks.tag_track_number")
		}).
		First(album, id.Value).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return spec.NewError(10, "couldn't find an album with that id")
	}
	sub := spec.NewResponse()
	sub.Album = spec.NewAlbumByTags(album, album.TagArtist)
	sub.Album.Tracks = make([]*spec.TrackChild, len(album.Tracks))
	for i, track := range album.Tracks {
		sub.Album.Tracks[i] = spec.NewTrackByTags(track, album)
	}
	return sub
}

// ServeGetAlbumListTwo handles the getAlbumList2 view.
// changes to this function should be reflected in in _by_folder.go's
// getAlbumList() function
func (c *Controller) ServeGetAlbumListTwo(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	listType, err := params.Get("type")
	if err != nil {
		return spec.NewError(10, "please provide a `type` parameter")
	}
	q := c.DB.DB
	switch listType {
	case "alphabeticalByArtist":
		q = q.Joins("JOIN artists ON albums.tag_artist_id=artists.id")
		q = q.Order("artists.name")
	case "alphabeticalByName":
		q = q.Order("tag_title")
	case "byYear":
		q = q.Where(
			"tag_year BETWEEN ? AND ?",
			params.GetOrInt("fromYear", 1800),
			params.GetOrInt("toYear", 2200))
		q = q.Order("tag_year")
	case "byGenre":
		q = q.Joins("JOIN genres ON albums.tag_genre_id=genres.id AND genres.name=?",
			params.GetOr("genre", "Unknown Genre"))
	case "frequent":
		user := r.Context().Value(CtxUser).(*db.User)
		q = q.Joins("JOIN plays ON albums.id=plays.album_id AND plays.user_id=?",
			user.ID)
		q = q.Order("plays.count DESC")
	case "newest":
		q = q.Order("modified_at DESC")
	case "random":
		q = q.Order(gorm.Expr("random()"))
	case "recent":
		user := r.Context().Value(CtxUser).(*db.User)
		q = q.Joins("JOIN plays ON albums.id=plays.album_id AND plays.user_id=?",
			user.ID)
		q = q.Order("plays.time DESC")
	default:
		return spec.NewError(10, "unknown value `%s` for parameter 'type'", listType)
	}
	var albums []*db.Album
	// TODO: think about removing this extra join to count number
	// of children. it might make sense to store that in the db
	q.
		Select("albums.*, count(tracks.id) child_count").
		Joins("LEFT JOIN tracks ON tracks.album_id=albums.id").
		Group("albums.id").
		Where("albums.tag_artist_id IS NOT NULL").
		Offset(params.GetOrInt("offset", 0)).
		Limit(params.GetOrInt("size", 10)).
		Preload("TagArtist").
		Find(&albums)
	sub := spec.NewResponse()
	sub.AlbumsTwo = &spec.Albums{
		List: make([]*spec.Album, len(albums)),
	}
	for i, album := range albums {
		sub.AlbumsTwo.List[i] = spec.NewAlbumByTags(album, album.TagArtist)
	}
	return sub
}

func (c *Controller) ServeSearchThree(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	query, err := params.Get("query")
	if err != nil {
		return spec.NewError(10, "please provide a `query` parameter")
	}
	query = fmt.Sprintf("%%%s%%",
		strings.TrimSuffix(query, "*"))
	results := &spec.SearchResultThree{}
	// ** begin search "artists"
	var artists []*db.Artist
	c.DB.
		Where("name LIKE ? OR name_u_dec LIKE ?",
			query, query).
		Offset(params.GetOrInt("artistOffset", 0)).
		Limit(params.GetOrInt("artistCount", 20)).
		Find(&artists)
	for _, a := range artists {
		results.Artists = append(results.Artists,
			spec.NewArtistByTags(a))
	}
	// ** begin search "albums"
	var albums []*db.Album
	c.DB.
		Preload("TagArtist").
		Where("tag_title LIKE ? OR tag_title_u_dec LIKE ?",
			query, query).
		Offset(params.GetOrInt("albumOffset", 0)).
		Limit(params.GetOrInt("albumCount", 20)).
		Find(&albums)
	for _, a := range albums {
		results.Albums = append(results.Albums,
			spec.NewAlbumByTags(a, a.TagArtist))
	}
	// ** begin search tracks
	var tracks []*db.Track
	c.DB.
		Preload("Album").
		Where("tag_title LIKE ? OR tag_title_u_dec LIKE ?",
			query, query).
		Offset(params.GetOrInt("songOffset", 0)).
		Limit(params.GetOrInt("songCount", 20)).
		Find(&tracks)
	for _, t := range tracks {
		results.Tracks = append(results.Tracks,
			spec.NewTrackByTags(t, t.Album))
	}
	sub := spec.NewResponse()
	sub.SearchResultThree = results
	return sub
}

func (c *Controller) ServeGetArtistInfoTwo(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	apiKey := c.DB.GetSetting("lastfm_api_key")
	if apiKey == "" {
		return spec.NewError(0, "please set ask your admin to set the last.fm api key")
	}
	artist := &db.Artist{}
	err = c.DB.
		Where("id=?", id.Value).
		Find(artist).
		Error
	if gorm.IsRecordNotFoundError(err) {
		return spec.NewError(70, "artist with id `%s` not found", id)
	}
	info, err := lastfm.ArtistGetInfo(apiKey, artist)
	if err != nil {
		return spec.NewError(0, "fetching artist info: %v", err)
	}
	sub := spec.NewResponse()
	sub.ArtistInfoTwo = &spec.ArtistInfo{
		Biography:     info.Bio.Summary,
		MusicBrainzID: info.MBID,
		LastFMURL:     info.URL,
	}
	for _, image := range info.Image {
		switch image.Size {
		case "small":
			sub.ArtistInfoTwo.SmallImageURL = image.Text
		case "medium":
			sub.ArtistInfoTwo.MediumImageURL = image.Text
		case "large":
			sub.ArtistInfoTwo.LargeImageURL = image.Text
		}
	}
	count := params.GetOrInt("count", 20)
	inclNotPresent := params.GetOrBool("includeNotPresent", false)
	for i, similarInfo := range info.Similar.Artists {
		if i == count {
			break
		}
		artist = &db.Artist{}
		err = c.DB.
			Select("artists.*, count(albums.id) album_count").
			Where("name=?", similarInfo.Name).
			Joins("LEFT JOIN albums ON artists.id=albums.tag_artist_id").
			Group("artists.id").
			Find(artist).
			Error
		if gorm.IsRecordNotFoundError(err) && !inclNotPresent {
			continue
		}
		similar := &spec.SimilarArtist{
			ID: &specid.ID{Type: specid.Artist, Value: -1},
		}
		if artist.ID != 0 {
			similar.ID = artist.SID()
		}
		similar.Name = similarInfo.Name
		similar.AlbumCount = artist.AlbumCount
		sub.ArtistInfoTwo.SimilarArtist = append(
			sub.ArtistInfoTwo.SimilarArtist, similar)
	}
	return sub
}

func (c *Controller) ServeGetGenres(r *http.Request) *spec.Response {
	var genres []*db.Genre
	c.DB.
		Select(`*,
			(SELECT count(id) FROM albums WHERE tag_genre_id=genres.id) album_count,
			(SELECT count(id) FROM tracks WHERE tag_genre_id=genres.id) track_count`).
		Group("genres.id").
		Find(&genres)
	sub := spec.NewResponse()
	sub.Genres = &spec.Genres{
		List: make([]*spec.Genre, len(genres)),
	}
	for i, genre := range genres {
		sub.Genres.List[i] = spec.NewGenre(genre)
	}
	return sub
}

func (c *Controller) ServeGetSongsByGenre(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	genre, err := params.Get("genre")
	if err != nil {
		return spec.NewError(10, "please provide an `genre` parameter")
	}
	// TODO: add musicFolderId parameter
	// (since 1.12.0) only return albums in the music folder with the given id
	var tracks []*db.Track
	c.DB.
		Joins("JOIN albums ON tracks.album_id=albums.id").
		Joins("JOIN genres ON tracks.tag_genre_id=genres.id AND genres.name=?", genre).
		Preload("Album").
		Offset(params.GetOrInt("offset", 0)).
		Limit(params.GetOrInt("count", 10)).
		Find(&tracks)
	sub := spec.NewResponse()
	sub.TracksByGenre = &spec.TracksByGenre{
		List: make([]*spec.TrackChild, len(tracks)),
	}
	for i, track := range tracks {
		sub.TracksByGenre.List[i] = spec.NewTrackByTags(track, track.Album)
	}
	return sub
}
