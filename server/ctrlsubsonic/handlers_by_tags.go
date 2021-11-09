package ctrlsubsonic

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/server/db"
	"go.senan.xyz/gonic/server/scrobble/lastfm"
)

func (c *Controller) ServeGetArtists(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	var artists []*db.Artist
	q := c.DB.
		Select("*, count(sub.id) album_count").
		Joins("LEFT JOIN albums sub ON artists.id=sub.tag_artist_id").
		Group("artists.id").
		Order("artists.name COLLATE NOCASE")
	if m := c.getMusicFolder(params); m != "" {
		q = q.Where("sub.root_dir=?", m)
	}
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(10, "error finding artists: %v", err)
	}
	// [a-z#] -> 27
	indexMap := make(map[string]*spec.Index, 27)
	resp := make([]*spec.Index, 0, 27)
	for _, artist := range artists {
		key := lowerUDecOrHash(artist.IndexName())
		if _, ok := indexMap[key]; !ok {
			indexMap[key] = &spec.Index{
				Name:    key,
				Artists: []*spec.Artist{},
			}
			resp = append(resp, indexMap[key])
		}
		indexMap[key].Artists = append(indexMap[key].Artists,
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
		Preload("Albums", func(db *gorm.DB) *gorm.DB {
			return db.
				Select("*, count(sub.id) child_count, sum(sub.length) duration").
				Joins("LEFT JOIN tracks sub ON albums.id=sub.album_id").
				Group("albums.id")
		}).
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
		Select("albums.*, count(tracks.id) child_count, sum(tracks.length) duration").
		Joins("LEFT JOIN tracks ON tracks.album_id=albums.id").
		Preload("TagArtist").
		Preload("Genres").
		Preload("Tracks", func(db *gorm.DB) *gorm.DB {
			return db.Order("tracks.tag_disc_number, tracks.tag_track_number")
		}).
		First(album, id.Value).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
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
		genre, _ := params.Get("genre")
		q = q.Joins("JOIN album_genres ON album_genres.album_id=albums.id")
		q = q.Joins("JOIN genres ON genres.id=album_genres.genre_id AND genres.name=?", genre)
	case "frequent":
		user := r.Context().Value(CtxUser).(*db.User)
		q = q.Joins("JOIN plays ON albums.id=plays.album_id AND plays.user_id=?",
			user.ID)
		q = q.Order("plays.count DESC")
	case "newest":
		q = q.Order("created_at DESC")
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
	if m := c.getMusicFolder(params); m != "" {
		q = q.Where("root_dir=?", m)
	}
	var albums []*db.Album
	// TODO: think about removing this extra join to count number
	// of children. it might make sense to store that in the db
	q.
		Select("albums.*, count(tracks.id) child_count, sum(tracks.length) duration").
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
	query = fmt.Sprintf("%%%s%%", strings.TrimSuffix(query, "*"))
	results := &spec.SearchResultThree{}

	// search "artists"
	var artists []*db.Artist
	q := c.DB.
		Where("name LIKE ? OR name_u_dec LIKE ?", query, query).
		Offset(params.GetOrInt("artistOffset", 0)).
		Limit(params.GetOrInt("artistCount", 20))
	if m := c.getMusicFolder(params); m != "" {
		q = q.
			Joins("JOIN albums ON albums.tag_artist_id=artists.id").
			Where("albums.root_dir=?", m)
	}
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(0, "find artists: %v", err)
	}
	for _, a := range artists {
		results.Artists = append(results.Artists, spec.NewArtistByTags(a))
	}

	// search "albums"
	var albums []*db.Album
	q = c.DB.
		Preload("TagArtist").
		Where("tag_title LIKE ? OR tag_title_u_dec LIKE ?", query, query).
		Offset(params.GetOrInt("albumOffset", 0)).
		Limit(params.GetOrInt("albumCount", 20))
	if m := c.getMusicFolder(params); m != "" {
		q = q.Where("root_dir=?", m)
	}
	if err := q.Find(&albums).Error; err != nil {
		return spec.NewError(0, "find albums: %v", err)
	}
	for _, a := range albums {
		results.Albums = append(results.Albums, spec.NewAlbumByTags(a, a.TagArtist))
	}

	// search tracks
	var tracks []*db.Track
	q = c.DB.
		Preload("Album").
		Where("tag_title LIKE ? OR tag_title_u_dec LIKE ?", query, query).
		Offset(params.GetOrInt("songOffset", 0)).
		Limit(params.GetOrInt("songCount", 20))
	if m := c.getMusicFolder(params); m != "" {
		q = q.
			Joins("JOIN albums ON albums.id=tracks.album_id").
			Where("albums.root_dir=?", m)
	}
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find tracks: %v", err)
	}
	for _, t := range tracks {
		results.Tracks = append(results.Tracks, spec.NewTrackByTags(t, t.Album))
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
	apiKey, _ := c.DB.GetSetting("lastfm_api_key")
	if apiKey == "" {
		sub := spec.NewResponse()
		sub.ArtistInfoTwo = &spec.ArtistInfo{}
		return sub
	}
	artist := &db.Artist{}
	err = c.DB.
		Where("id=?", id.Value).
		Find(artist).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
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
		if errors.Is(err, gorm.ErrRecordNotFound) && !inclNotPresent {
			continue
		}
		similar := &spec.SimilarArtist{
			ID: &specid.ID{},
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
			(SELECT count(1) FROM album_genres WHERE genre_id=genres.id) album_count,
			(SELECT count(1) FROM track_genres WHERE genre_id=genres.id) track_count`).
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
	var tracks []*db.Track
	q := c.DB.
		Joins("JOIN albums ON tracks.album_id=albums.id").
		Joins("JOIN track_genres ON track_genres.track_id=tracks.id").
		Joins("JOIN genres ON track_genres.genre_id=genres.id AND genres.name=?", genre).
		Preload("Album").
		Offset(params.GetOrInt("offset", 0)).
		Limit(params.GetOrInt("count", 10))
	if m := c.getMusicFolder(params); m != "" {
		q = q.Where("albums.root_dir=?", m)
	}
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "error finding tracks: %v", err)
	}
	sub := spec.NewResponse()
	sub.TracksByGenre = &spec.TracksByGenre{
		List: make([]*spec.TrackChild, len(tracks)),
	}
	for i, track := range tracks {
		sub.TracksByGenre.List[i] = spec.NewTrackByTags(track, track.Album)
	}
	return sub
}
