package ctrlsubsonic

import (
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/handlerutil"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

func (c *Controller) ServeGetArtists(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	var artists []*db.Artist
	q := c.dbc.
		Select("*, count(album_artists.album_id) album_count").
		Joins("JOIN album_artists ON album_artists.artist_id=artists.id").
		Preload("ArtistStar", "user_id=?", user.ID).
		Preload("ArtistRating", "user_id=?", user.ID).
		Preload("Info").
		Group("artists.id").
		Order("artists.name COLLATE NOCASE")
	if m := getMusicFolder(c.musicPaths, params); m != "" {
		q = q.
			Joins("JOIN albums ON albums.id=album_artists.album_id").
			Where("albums.root_dir=?", m)
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
		indexMap[key].Artists = append(indexMap[key].Artists, spec.NewArtistByTags(artist))
	}
	sub := spec.NewResponse()
	sub.Artists = &spec.Artists{
		List: resp,
	}
	return sub
}

func (c *Controller) ServeGetArtist(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	var artist db.Artist
	c.dbc.
		Preload("Appearances", func(db *gorm.DB) *gorm.DB {
			return db.
				Select("*, count(sub.id) child_count, sum(sub.length) duration").
				Joins("LEFT JOIN tracks sub ON albums.id=sub.album_id").
				Order("albums.right_path").
				Group("albums.id")
		}).
		Preload("Appearances.Artists").
		Preload("Appearances.Genres").
		Preload("Info").
		Preload("ArtistStar", "user_id=?", user.ID).
		Preload("ArtistRating", "user_id=?", user.ID).
		First(&artist, id.Value)

	sub := spec.NewResponse()
	sub.Artist = spec.NewArtistByTags(&artist)
	sub.Artist.Albums = make([]*spec.Album, len(artist.Appearances))
	for i, album := range artist.Appearances {
		sub.Artist.Albums[i] = spec.NewAlbumByTags(album, album.Artists)
	}
	sub.Artist.AlbumCount = len(artist.Appearances)
	return sub
}

func (c *Controller) ServeGetAlbum(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	album := &db.Album{}
	err = c.dbc.
		Select("albums.*, count(tracks.id) child_count, sum(tracks.length) duration").
		Joins("LEFT JOIN tracks ON tracks.album_id=albums.id").
		Preload("Artists").
		Preload("Genres").
		Preload("Tracks", func(db *gorm.DB) *gorm.DB {
			return db.
				Order("tracks.tag_disc_number, tracks.tag_track_number").
				Preload("Artists").
				Preload("TrackStar", "user_id=?", user.ID).
				Preload("TrackRating", "user_id=?", user.ID)
		}).
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID).
		Preload("Play", "user_id=?", user.ID).
		First(album, id.Value).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(70, "couldn't find an album with that id")
	}
	sub := spec.NewResponse()
	sub.Album = spec.NewAlbumByTags(album, album.Artists)
	sub.Album.Tracks = make([]*spec.TrackChild, len(album.Tracks))

	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, params.GetOr("c", ""))

	for i, track := range album.Tracks {
		sub.Album.Tracks[i] = spec.NewTrackByTags(track, album)
		sub.Album.Tracks[i].TranscodeMeta = transcodeMeta
	}
	return sub
}

// ServeGetAlbumListTwo handles the getAlbumList2 view.
// changes to this function should be reflected in _by_folder.go's
// getAlbumList() function
func (c *Controller) ServeGetAlbumListTwo(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	listType, err := params.Get("type")
	if err != nil {
		return spec.NewError(10, "please provide a `type` parameter")
	}
	q := c.dbc.DB
	switch listType {
	case "alphabeticalByArtist":
		q = q.Joins("JOIN artists ON artists.id=album_artists.artist_id")
		q = q.Order("artists.name")
	case "alphabeticalByName":
		q = q.Order("tag_title")
	case "byYear":
		y1, y2 := params.GetOrInt("fromYear", 1800),
			params.GetOrInt("toYear", 2200)
		// support some clients sending wrong order like DSub
		q = q.Where("tag_year BETWEEN ? AND ?", min(y1, y2), max(y1, y2))
		q = q.Order("tag_year DESC")
	case "byGenre":
		genre, _ := params.Get("genre")
		q = q.Joins("JOIN album_genres ON album_genres.album_id=albums.id")
		q = q.Joins("JOIN genres ON genres.id=album_genres.genre_id AND genres.name=?", genre)
		q = q.Order("tag_title")
	case "frequent":
		user := r.Context().Value(CtxUser).(*db.User)
		q = q.Joins("JOIN plays ON albums.id=plays.album_id AND plays.user_id=?", user.ID)
		q = q.Order("plays.length DESC")
	case "newest":
		q = q.Order("created_at DESC")
	case "random":
		q = q.Order(gorm.Expr("random()"))
	case "recent":
		q = q.Joins("JOIN plays ON albums.id=plays.album_id AND plays.user_id=?", user.ID)
		q = q.Order("plays.time DESC")
	case "starred":
		q = q.Joins("JOIN album_stars ON albums.id=album_stars.album_id AND album_stars.user_id=?", user.ID)
		q = q.Order("tag_title")
	case "highest":
		q = q.Joins("JOIN album_ratings ON album_ratings.album_id=albums.id AND album_ratings.user_id=?", user.ID)
		q = q.Order("album_ratings.rating DESC")
	default:
		return spec.NewError(10, "unknown value %q for parameter 'type'", listType)
	}
	if m := getMusicFolder(c.musicPaths, params); m != "" {
		q = q.Where("root_dir=?", m)
	}
	var albums []*db.Album
	// TODO: think about removing this extra join to count number
	// of children. it might make sense to store that in the db
	q.
		Select("albums.*, count(tracks.id) child_count, sum(tracks.length) duration").
		Joins("LEFT JOIN tracks ON tracks.album_id=albums.id").
		Group("albums.id").
		Joins("JOIN album_artists ON album_artists.album_id=albums.id").
		Offset(params.GetOrInt("offset", 0)).
		Limit(params.GetOrInt("size", 10)).
		Preload("Artists").
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID).
		Preload("Play", "user_id=?", user.ID).
		Find(&albums)
	sub := spec.NewResponse()
	sub.AlbumsTwo = &spec.Albums{
		List: make([]*spec.Album, len(albums)),
	}
	for i, album := range albums {
		sub.AlbumsTwo.List[i] = spec.NewAlbumByTags(album, album.Artists)
	}
	return sub
}

func (c *Controller) ServeSearchThree(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	query, err := params.Get("query")
	var queries []string
	if err != nil {
		return spec.NewError(10, "please provide a `query` parameter")
	}
	for _, s := range strings.Fields(query) {
		queries = append(queries, fmt.Sprintf("%%%s%%", strings.Trim(s, `*"'`)))
	}

	results := &spec.SearchResultThree{}

	// search artists
	var artists []*db.Artist
	q := c.dbc.
		Select("*, count(albums.id) album_count").
		Group("artists.id")
	for _, s := range queries {
		q = q.Where(`name LIKE ? OR name_u_dec LIKE ?`, s, s)
	}
	q = q.
		Joins("JOIN album_artists ON album_artists.artist_id=artists.id").
		Joins("JOIN albums ON albums.id=album_artists.album_id").
		Preload("ArtistStar", "user_id=?", user.ID).
		Preload("ArtistRating", "user_id=?", user.ID).
		Preload("Info").
		Offset(params.GetOrInt("artistOffset", 0)).
		Limit(params.GetOrInt("artistCount", 20))
	if m := getMusicFolder(c.musicPaths, params); m != "" {
		q = q.Where("albums.root_dir=?", m)
	}
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(0, "find artists: %v", err)
	}
	for _, a := range artists {
		results.Artists = append(results.Artists, spec.NewArtistByTags(a))
	}

	// search albums
	var albums []*db.Album
	q = c.dbc.
		Preload("Artists").
		Preload("Genres").
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID).
		Preload("Play", "user_id=?", user.ID)
	for _, s := range queries {
		q = q.Where(`tag_title LIKE ? OR tag_title_u_dec LIKE ?`, s, s)
	}
	q = q.
		Offset(params.GetOrInt("albumOffset", 0)).
		Limit(params.GetOrInt("albumCount", 20))
	if m := getMusicFolder(c.musicPaths, params); m != "" {
		q = q.Where("root_dir=?", m)
	}
	if err := q.Find(&albums).Error; err != nil {
		return spec.NewError(0, "find albums: %v", err)
	}
	for _, a := range albums {
		results.Albums = append(results.Albums, spec.NewAlbumByTags(a, a.Artists))
	}

	// search tracks
	var tracks []*db.Track
	q = c.dbc.
		Preload("Album").
		Preload("Album.Artists").
		Preload("Genres").
		Preload("Artists").
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID)
	for _, s := range queries {
		q = q.Where(`tracks.tag_title LIKE ? OR tracks.tag_title_u_dec LIKE ?`, s, s)
	}
	q = q.Offset(params.GetOrInt("songOffset", 0)).
		Limit(params.GetOrInt("songCount", 20))
	if m := getMusicFolder(c.musicPaths, params); m != "" {
		q = q.
			Joins("JOIN albums ON albums.id=tracks.album_id").
			Where("albums.root_dir=?", m)
	}
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find tracks: %v", err)
	}

	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, params.GetOr("c", ""))

	for _, t := range tracks {
		track := spec.NewTrackByTags(t, t.Album)
		track.TranscodeMeta = transcodeMeta
		results.Tracks = append(results.Tracks, track)
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

	var artist db.Artist
	err = c.dbc.
		Where("id=?", id.Value).
		Find(&artist).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(70, "artist with id %q not found", id)
	}

	sub := spec.NewResponse()
	sub.ArtistInfoTwo = &spec.ArtistInfo{}

	info, err := c.artistInfoCache.GetOrLookup(r.Context(), artist.ID)
	if err != nil {
		log.Printf("error fetching artist info from lastfm: %v", err)
		return sub
	}

	sub.ArtistInfoTwo.Biography = spec.CleanExternalText(info.Biography)
	sub.ArtistInfoTwo.MusicBrainzID = info.MusicBrainzID
	sub.ArtistInfoTwo.LastFMURL = info.LastFMURL

	sub.ArtistInfoTwo.SmallImageURL = c.genArtistCoverURL(r, &artist, 64)
	sub.ArtistInfoTwo.MediumImageURL = c.genArtistCoverURL(r, &artist, 126)
	sub.ArtistInfoTwo.LargeImageURL = c.genArtistCoverURL(r, &artist, 256)

	if info.ImageURL != "" {
		sub.ArtistInfoTwo.SmallImageURL = info.ImageURL
		sub.ArtistInfoTwo.MediumImageURL = info.ImageURL
		sub.ArtistInfoTwo.LargeImageURL = info.ImageURL
		sub.ArtistInfoTwo.ArtistImageURL = info.ImageURL
	}

	count := params.GetOrInt("count", 20)
	inclNotPresent := params.GetOrBool("includeNotPresent", false)

	for i, similarName := range info.GetSimilarArtists() {
		if i == count {
			break
		}
		var artist db.Artist
		err = c.dbc.
			Preload("Info").
			Select("artists.*, count(albums.id) album_count").
			Where("name=?", similarName).
			Joins("LEFT JOIN artist_appearances ON artist_appearances.artist_id=artists.id").
			Joins("LEFT JOIN albums ON albums.id=artist_appearances.album_id").
			Group("artists.id").
			Find(&artist).
			Error
		if errors.Is(err, gorm.ErrRecordNotFound) && !inclNotPresent {
			continue
		}

		if artist.ID == 0 {
			// add a very limited artist, since we don't have everything with `inclNotPresent`
			sub.ArtistInfoTwo.Similar = append(sub.ArtistInfoTwo.Similar, &spec.Artist{
				ID:   &specid.ID{},
				Name: similarName,
			})
			continue
		}

		sub.ArtistInfoTwo.Similar = append(sub.ArtistInfoTwo.Similar, spec.NewArtistByTags(&artist))
	}

	return sub
}

func (c *Controller) ServeGetAlbumInfoTwo(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}

	var album db.Album
	err = c.dbc.
		Where("id=?", id.Value).
		Find(&album).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(70, "album with id %q not found", id)
	}

	sub := spec.NewResponse()
	sub.AlbumInfo = &spec.AlbumInfo{}

	info, err := c.albumInfoCache.GetOrLookup(r.Context(), album.ID)
	if err != nil {
		log.Printf("error fetching album info from lastfm: %v", err)
		return sub
	}

	sub.AlbumInfo.Notes = spec.CleanExternalText(info.Notes)
	sub.AlbumInfo.MusicBrainzID = info.MusicBrainzID
	sub.AlbumInfo.LastFMURL = info.LastFMURL

	if _, err := uuid.Parse(album.TagBrainzID); err == nil {
		sub.AlbumInfo.MusicBrainzID = album.TagBrainzID // prefer db musicbrainz ID over lastfm's
	}

	return sub
}

func (c *Controller) ServeGetGenres(_ *http.Request) *spec.Response {
	var genres []*db.Genre
	c.dbc.
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
	user := r.Context().Value(CtxUser).(*db.User)
	genre, err := params.Get("genre")
	if err != nil {
		return spec.NewError(10, "please provide an `genre` parameter")
	}
	var tracks []*db.Track
	q := c.dbc.
		Joins("JOIN albums ON tracks.album_id=albums.id").
		Joins("JOIN track_genres ON track_genres.track_id=tracks.id").
		Joins("JOIN genres ON track_genres.genre_id=genres.id AND genres.name=?", genre).
		Preload("Album").
		Preload("Album.Artists").
		Preload("Artists").
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID).
		Offset(params.GetOrInt("offset", 0)).
		Limit(params.GetOrInt("count", 10))
	if m := getMusicFolder(c.musicPaths, params); m != "" {
		q = q.Where("albums.root_dir=?", m)
	}
	q = q.Group("tracks.id")
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "error finding tracks: %v", err)
	}
	sub := spec.NewResponse()
	sub.TracksByGenre = &spec.TracksByGenre{
		List: make([]*spec.TrackChild, len(tracks)),
	}

	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, params.GetOr("c", ""))

	for i, t := range tracks {
		sub.TracksByGenre.List[i] = spec.NewTrackByTags(t, t.Album)
		sub.TracksByGenre.List[i].TranscodeMeta = transcodeMeta
	}

	return sub
}

func (c *Controller) ServeGetStarredTwo(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(params.Params)

	results := &spec.StarredTwo{}

	// artists
	var artists []*db.Artist
	q := c.dbc.
		Select("artists.*, count(albums.id) album_count").
		Joins("JOIN artist_stars ON artist_stars.artist_id=artists.id").
		Where("artist_stars.user_id=?", user.ID).
		Joins("JOIN artist_appearances ON artist_appearances.artist_id=artists.id").
		Joins("JOIN albums ON albums.id=artist_appearances.album_id").
		Order("artist_stars.star_date DESC").
		Preload("ArtistStar", "user_id=?", user.ID).
		Preload("ArtistRating", "user_id=?", user.ID).
		Preload("Info").
		Group("artists.id")
	if m := getMusicFolder(c.musicPaths, params); m != "" {
		q = q.Where("albums.root_dir=?", m)
	}
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(0, "find artists: %v", err)
	}
	for _, a := range artists {
		results.Artists = append(results.Artists, spec.NewArtistByTags(a))
	}

	// albums
	var albums []*db.Album
	q = c.dbc.
		Joins("JOIN album_stars ON album_stars.album_id=albums.id").
		Where("album_stars.user_id=?", user.ID).
		Order("album_stars.star_date DESC").
		Preload("Artists").
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID).
		Preload("Play", "user_id=?", user.ID)
	if m := getMusicFolder(c.musicPaths, params); m != "" {
		q = q.Where("albums.root_dir=?", m)
	}
	if err := q.Find(&albums).Error; err != nil {
		return spec.NewError(0, "find albums: %v", err)
	}
	for _, a := range albums {
		results.Albums = append(results.Albums, spec.NewAlbumByTags(a, a.Artists))
	}

	// tracks
	var tracks []*db.Track
	q = c.dbc.
		Joins("JOIN track_stars ON tracks.id=track_stars.track_id").
		Where("track_stars.user_id=?", user.ID).
		Order("track_stars.star_date DESC").
		Preload("Album").
		Preload("Album.Artists").
		Preload("Artists").
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID)
	if m := getMusicFolder(c.musicPaths, params); m != "" {
		q = q.
			Joins("JOIN albums ON albums.id=tracks.album_id").
			Where("albums.root_dir=?", m)
	}
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find tracks: %v", err)
	}

	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, params.GetOr("c", ""))

	for _, t := range tracks {
		track := spec.NewTrackByTags(t, t.Album)
		track.TranscodeMeta = transcodeMeta
		results.Tracks = append(results.Tracks, track)
	}

	sub := spec.NewResponse()
	sub.StarredTwo = results
	return sub
}

func (c *Controller) genArtistCoverURL(r *http.Request, artist *db.Artist, size int) string {
	coverURL, _ := url.Parse(handlerutil.BaseURL(r))
	coverURL.Path = c.resolveProxyPath("/rest/getCoverArt")

	query := r.URL.Query()
	query.Set("id", artist.SID().String())
	query.Set("size", strconv.Itoa(size))
	coverURL.RawQuery = query.Encode()

	return coverURL.String()
}

func (c *Controller) ServeGetTopSongs(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	count := params.GetOrInt("count", 10)
	artistName, err := params.Get("artist")
	if err != nil {
		return spec.NewError(10, "please provide an `artist` parameter")
	}
	var artist db.Artist
	if err := c.dbc.Where("name=?", artistName).Find(&artist).Error; err != nil {
		return spec.NewError(0, "finding artist by name: %v", err)
	}

	info, err := c.artistInfoCache.GetOrLookup(r.Context(), artist.ID)
	if err != nil {
		log.Printf("error fetching artist info from lastfm: %v", err)
		return spec.NewResponse()
	}

	sub := spec.NewResponse()
	sub.TopSongs = &spec.TopSongs{
		Tracks: make([]*spec.TrackChild, 0),
	}

	topTrackNames := info.GetTopTracks()
	if len(topTrackNames) == 0 {
		return sub
	}

	var tracks []*db.Track
	err = c.dbc.
		Where("tracks.tag_title IN (?)", topTrackNames).
		Joins("JOIN track_artists ON track_artists.track_id=tracks.id").
		Joins("JOIN artists ON artists.id=track_artists.artist_id").
		Where("artists.id=?", artist.ID).
		Preload("Album").
		Preload("Artists").
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID).
		Group("tracks.id").
		Limit(count).
		Find(&tracks).
		Error
	if err != nil {
		return spec.NewError(0, "error finding tracks: %v", err)
	}
	if len(tracks) == 0 {
		return sub
	}

	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, params.GetOr("c", ""))

	for _, track := range tracks {
		tc := spec.NewTrackByTags(track, track.Album)
		tc.TranscodeMeta = transcodeMeta
		sub.TopSongs.Tracks = append(sub.TopSongs.Tracks, tc)
	}
	return sub
}

func (c *Controller) ServeGetSimilarSongs(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	count := params.GetOrInt("count", 10)
	id, err := params.GetID("id")
	if err != nil || id.Type != specid.Track {
		return spec.NewError(10, "please provide an track `id` parameter")
	}

	var track db.Track
	err = c.dbc.
		Preload("Album").
		Where("id=?", id.Value).
		First(&track).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(70, "couldn't find a track with that id")
	}

	similarTracks, err := c.lastFMClient.TrackGetSimilarTracks(track.TagTrackArtist, track.TagTitle)
	if err != nil {
		log.Printf("error fetching similar songs from lastfm: %v", err)
		return spec.NewResponse()
	}

	if len(similarTracks.Tracks) == 0 {
		return spec.NewError(70, "no similar songs found for track: %v", track.TagTitle)
	}

	similarTrackNames := make([]string, len(similarTracks.Tracks))
	for i, t := range similarTracks.Tracks {
		similarTrackNames[i] = t.Name
	}

	var tracks []*db.Track
	err = c.dbc.
		Select("tracks.*").
		Preload("Album").
		Preload("Artists").
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID).
		Where("tracks.tag_title IN (?)", similarTrackNames).
		Order(gorm.Expr("random()")).
		Limit(count).
		Find(&tracks).
		Error
	if err != nil {
		return spec.NewError(0, "error finding tracks: %v", err)
	}
	if len(tracks) == 0 {
		return spec.NewError(70, "no similar song could be match with collection in database: %v", track.TagTitle)
	}

	sub := spec.NewResponse()
	sub.SimilarSongs = &spec.SimilarSongs{
		Tracks: make([]*spec.TrackChild, len(tracks)),
	}

	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, params.GetOr("c", ""))

	for i, track := range tracks {
		sub.SimilarSongs.Tracks[i] = spec.NewTrackByTags(track, track.Album)
		sub.SimilarSongs.Tracks[i].TranscodeMeta = transcodeMeta
	}
	return sub
}

func (c *Controller) ServeGetSimilarSongsTwo(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	count := params.GetOrInt("count", 10)
	id, err := params.GetID("id")
	if err != nil || id.Type != specid.Artist {
		return spec.NewError(10, "please provide an artist `id` parameter")
	}

	var artist db.Artist
	err = c.dbc.
		Where("id=?", id.Value).
		First(&artist).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(70, "artist with id %q not found", id)
	}

	similarArtists, err := c.lastFMClient.ArtistGetSimilar(artist.Name)
	if err != nil {
		log.Printf("error fetching artist info from lastfm: %v", err)
		return spec.NewResponse()
	}
	if len(similarArtists.Artists) == 0 {
		return spec.NewError(0, "no similar artist found for: %v", artist.Name)
	}

	artistNames := make([]string, len(similarArtists.Artists))
	for i, similarArtist := range similarArtists.Artists {
		artistNames[i] = similarArtist.Name
	}

	var tracks []*db.Track
	err = c.dbc.
		Preload("Album").
		Preload("Artists").
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID).
		Joins("JOIN track_artists ON track_artists.track_id=tracks.id").
		Joins("JOIN artists ON artists.id=track_artists.artist_id").
		Where("artists.name IN (?)", artistNames).
		Order(gorm.Expr("random()")).
		Group("tracks.id").
		Limit(count).
		Find(&tracks).
		Error
	if err != nil {
		return spec.NewError(0, "error finding tracks: %v", err)
	}
	if len(tracks) == 0 {
		return spec.NewError(70, "no similar song could be match with collection in database: %v", artist.Name)
	}

	sub := spec.NewResponse()
	sub.SimilarSongsTwo = &spec.SimilarSongsTwo{
		Tracks: make([]*spec.TrackChild, len(tracks)),
	}

	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, params.GetOr("c", ""))
	for i, track := range tracks {
		sub.SimilarSongsTwo.Tracks[i] = spec.NewTrackByTags(track, track.Album)
		sub.SimilarSongsTwo.Tracks[i].TranscodeMeta = transcodeMeta
	}
	return sub
}

func starIDsOfType(p params.Params, typ specid.IDT) []int {
	var ids []specid.ID
	ids = append(ids, p.GetOrIDList("id", nil)...)
	ids = append(ids, p.GetOrIDList("albumId", nil)...)
	ids = append(ids, p.GetOrIDList("artistId", nil)...)

	var out []int
	for _, id := range ids {
		if id.Type != typ {
			continue
		}
		out = append(out, id.Value)
	}
	return out
}

func (c *Controller) ServeStar(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)

	stardate := time.Now()
	for _, id := range starIDsOfType(params, specid.Album) {
		var albumstar db.AlbumStar
		_ = c.dbc.Where("user_id=? AND album_id=?", user.ID, id).First(&albumstar).Error
		albumstar.UserID = user.ID
		albumstar.AlbumID = id
		albumstar.StarDate = stardate
		if err := c.dbc.Save(&albumstar).Error; err != nil {
			return spec.NewError(0, "save album star: %v", err)
		}
	}

	for _, id := range starIDsOfType(params, specid.Artist) {
		var artiststar db.ArtistStar
		_ = c.dbc.Where("user_id=? AND artist_id=?", user.ID, id).First(&artiststar).Error
		artiststar.UserID = user.ID
		artiststar.ArtistID = id
		artiststar.StarDate = stardate
		if err := c.dbc.Save(&artiststar).Error; err != nil {
			return spec.NewError(0, "save artist star: %v", err)
		}
	}

	for _, id := range starIDsOfType(params, specid.Track) {
		var trackstar db.TrackStar
		_ = c.dbc.Where("user_id=? AND track_id=?", user.ID, id).First(&trackstar).Error
		trackstar.UserID = user.ID
		trackstar.TrackID = id
		trackstar.StarDate = stardate
		if err := c.dbc.Save(&trackstar).Error; err != nil {
			return spec.NewError(0, "save track star: %v", err)
		}
	}

	return spec.NewResponse()
}

func (c *Controller) ServeUnstar(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)

	for _, id := range starIDsOfType(params, specid.Album) {
		if err := c.dbc.Where("user_id=? AND album_id=?", user.ID, id).Delete(db.AlbumStar{}).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(0, "delete album star: %v", err)
		}
	}

	for _, id := range starIDsOfType(params, specid.Artist) {
		if err := c.dbc.Where("user_id=? AND artist_id=?", user.ID, id).Delete(db.ArtistStar{}).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(0, "delete artist star: %v", err)
		}
	}

	for _, id := range starIDsOfType(params, specid.Track) {
		if err := c.dbc.Where("user_id=? AND track_id=?", user.ID, id).Delete(db.TrackStar{}).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(0, "delete track star: %v", err)
		}
	}

	return spec.NewResponse()
}

//nolint:gocyclo // we could probably simplify this with some interfaces or generics. but it's fine for now
func (c *Controller) ServeSetRating(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide a valid id")
	}
	rating, err := params.GetInt("rating")
	if err != nil || rating < 0 || rating > 5 {
		return spec.NewError(10, "please provide a valid rating")
	}

	user := r.Context().Value(CtxUser).(*db.User)

	switch id.Type {
	case specid.Album:
		var album db.Album
		err := c.dbc.Where("id=?", id.Value).First(&album).Error
		if err != nil {
			return spec.NewError(0, "fetch album: %v", err)
		}
		var albumRating db.AlbumRating
		if err := c.dbc.Where("user_id=? AND album_id=?", user.ID, id.Value).First(&albumRating).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(0, "fetch album rating: %v", err)
		}
		switch {
		case rating == 0 && albumRating.AlbumID == album.ID:
			if err := c.dbc.Delete(&albumRating).Error; err != nil {
				return spec.NewError(0, "delete album rating: %v", err)
			}
		case rating > 0:
			albumRating.UserID = user.ID
			albumRating.AlbumID = id.Value
			albumRating.Rating = rating
			if err := c.dbc.Save(&albumRating).Error; err != nil {
				return spec.NewError(0, "save album rating: %v", err)
			}
		}
		var averageRating float64
		if err := c.dbc.Model(db.AlbumRating{}).Select("coalesce(avg(rating), 0)").Where("album_id=?", id.Value).Row().Scan(&averageRating); err != nil {
			return spec.NewError(0, "find average album rating: %v", err)
		}
		album.AverageRating = math.Trunc(averageRating*100) / 100
		if err := c.dbc.Save(&album).Error; err != nil {
			return spec.NewError(0, "save album: %v", err)
		}
	case specid.Artist:
		var artist db.Artist
		err := c.dbc.Where("id=?", id.Value).First(&artist).Error
		if err != nil {
			return spec.NewError(0, "fetch artist: %v", err)
		}
		var artistRating db.ArtistRating
		if err := c.dbc.Where("user_id=? AND artist_id=?", user.ID, id.Value).First(&artistRating).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(0, "fetch artist rating: %v", err)
		}
		switch {
		case rating == 0 && artistRating.ArtistID == artist.ID:
			if err := c.dbc.Delete(&artistRating).Error; err != nil {
				return spec.NewError(0, "delete artist rating: %v", err)
			}
		case rating > 0:
			artistRating.UserID = user.ID
			artistRating.ArtistID = id.Value
			artistRating.Rating = rating
			if err := c.dbc.Save(&artistRating).Error; err != nil {
				return spec.NewError(0, "save artist rating: %v", err)
			}
		}
		var averageRating float64
		if err := c.dbc.Model(db.ArtistRating{}).Select("coalesce(avg(rating), 0)").Where("artist_id=?", id.Value).Row().Scan(&averageRating); err != nil {
			return spec.NewError(0, "find average artist rating: %v", err)
		}
		artist.AverageRating = math.Trunc(averageRating*100) / 100
		if err := c.dbc.Save(&artist).Error; err != nil {
			return spec.NewError(0, "save artist: %v", err)
		}
	case specid.Track:
		var track db.Track
		err := c.dbc.Where("id=?", id.Value).First(&track).Error
		if err != nil {
			return spec.NewError(0, "fetch track: %v", err)
		}
		var trackRating db.TrackRating
		if err := c.dbc.Where("user_id=? AND track_id=?", user.ID, id.Value).First(&trackRating).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(0, "fetch track rating: %v", err)
		}
		switch {
		case rating == 0 && trackRating.TrackID == track.ID:
			if err := c.dbc.Delete(&trackRating).Error; err != nil {
				return spec.NewError(0, "delete track rating: %v", err)
			}
		case rating > 0:
			trackRating.UserID = user.ID
			trackRating.TrackID = id.Value
			trackRating.Rating = rating
			if err := c.dbc.Save(&trackRating).Error; err != nil {
				return spec.NewError(0, "save track rating: %v", err)
			}
		}
		var averageRating float64
		if err := c.dbc.Model(db.TrackRating{}).Select("coalesce(avg(rating), 0)").Where("track_id=?", id.Value).Row().Scan(&averageRating); err != nil {
			return spec.NewError(0, "find average track rating: %v", err)
		}
		track.AverageRating = math.Trunc(averageRating*100) / 100
		if err := c.dbc.Save(&track).Error; err != nil {
			return spec.NewError(0, "save track: %v", err)
		}
	default:
		return spec.NewError(0, "non-album non-artist non-track id cannot be rated")
	}

	return spec.NewResponse()
}
