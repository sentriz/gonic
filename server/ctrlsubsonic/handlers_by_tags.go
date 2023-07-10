package ctrlsubsonic

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

func (c *Controller) ServeGetArtists(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	var artists []*db.Artist
	q := c.DB.
		Select("*, count(sub.id) album_count").
		Joins("LEFT JOIN albums sub ON artists.id=sub.tag_artist_id").
		Preload("ArtistStar", "user_id=?", user.ID).
		Preload("ArtistRating", "user_id=?", user.ID).
		Group("artists.id").
		Order("artists.name COLLATE NOCASE")
	if m := getMusicFolder(c.MusicPaths, params); m != "" {
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
	artist := &db.Artist{}
	c.DB.
		Preload("Albums", func(db *gorm.DB) *gorm.DB {
			return db.
				Select("*, count(sub.id) child_count, sum(sub.length) duration").
				Joins("LEFT JOIN tracks sub ON albums.id=sub.album_id").
				Preload("AlbumStar", "user_id=?", user.ID).
				Preload("AlbumRating", "user_id=?", user.ID).
				Order("albums.right_path").
				Group("albums.id")
		}).
		Preload("ArtistStar", "user_id=?", user.ID).
		Preload("ArtistRating", "user_id=?", user.ID).
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
	user := r.Context().Value(CtxUser).(*db.User)
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
			return db.
				Order("tracks.tag_disc_number, tracks.tag_track_number").
				Preload("TrackStar", "user_id=?", user.ID).
				Preload("TrackRating", "user_id=?", user.ID)
		}).
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID).
		First(album, id.Value).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(10, "couldn't find an album with that id")
	}
	sub := spec.NewResponse()
	sub.Album = spec.NewAlbumByTags(album, album.TagArtist)
	sub.Album.Tracks = make([]*spec.TrackChild, len(album.Tracks))

	transcodeMIME, transcodeSuffix := streamGetTransPrefProfile(c.DB, user.ID, params.GetOr("c", ""))

	for i, track := range album.Tracks {
		sub.Album.Tracks[i] = spec.NewTrackByTags(track, album)
		sub.Album.Tracks[i].TranscodedContentType = transcodeMIME
		sub.Album.Tracks[i].TranscodedSuffix = transcodeSuffix
	}
	return sub
}

// ServeGetAlbumListTwo handles the getAlbumList2 view.
// changes to this function should be reflected in in _by_folder.go's
// getAlbumList() function
func (c *Controller) ServeGetAlbumListTwo(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
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
		fromYear := params.GetOrInt("fromYear", 1800)
		toYear := params.GetOrInt("toYear", 2200)
		if fromYear > toYear {
			q = q.Where("tag_year BETWEEN ? AND ?", toYear, fromYear)
			q = q.Order("tag_year DESC")
		} else {
			q = q.Where("tag_year BETWEEN ? AND ?", fromYear, toYear)
			q = q.Order("tag_year")
		}
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
	default:
		return spec.NewError(10, "unknown value `%s` for parameter 'type'", listType)
	}
	if m := getMusicFolder(c.MusicPaths, params); m != "" {
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
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID).
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
	q := c.DB.
		Select("*, count(albums.id) album_count").
		Group("artists.id")
	for _, s := range queries {
		q = q.Where(`name LIKE ? OR name_u_dec LIKE ?`, s, s)
	}
	q = q.Joins("JOIN albums ON albums.tag_artist_id=artists.id").
		Preload("ArtistStar", "user_id=?", user.ID).
		Preload("ArtistRating", "user_id=?", user.ID).
		Offset(params.GetOrInt("artistOffset", 0)).
		Limit(params.GetOrInt("artistCount", 20))
	if m := getMusicFolder(c.MusicPaths, params); m != "" {
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
	q = c.DB.
		Preload("TagArtist").
		Preload("Genres").
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID)
	for _, s := range queries {
		q = q.Where(`tag_title LIKE ? OR tag_title_u_dec LIKE ?`, s, s)
	}
	q = q.Offset(params.GetOrInt("albumOffset", 0)).
		Limit(params.GetOrInt("albumCount", 20))
	if m := getMusicFolder(c.MusicPaths, params); m != "" {
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
		Preload("Album.TagArtist").
		Preload("Genres").
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID)
	for _, s := range queries {
		q = q.Where(`tag_title LIKE ? OR tag_title_u_dec LIKE ?`, s, s)
	}
	q = q.Offset(params.GetOrInt("songOffset", 0)).
		Limit(params.GetOrInt("songCount", 20))
	if m := getMusicFolder(c.MusicPaths, params); m != "" {
		q = q.
			Joins("JOIN albums ON albums.id=tracks.album_id").
			Where("albums.root_dir=?", m)
	}
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find tracks: %v", err)
	}

	transcodeMIME, transcodeSuffix := streamGetTransPrefProfile(c.DB, user.ID, params.GetOr("c", ""))

	for _, t := range tracks {
		track := spec.NewTrackByTags(t, t.Album)
		track.TranscodedContentType = transcodeMIME
		track.TranscodedSuffix = transcodeSuffix
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
	err = c.DB.
		Where("id=?", id.Value).
		Find(&artist).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(70, "artist with id `%s` not found", id)
	}

	sub := spec.NewResponse()
	sub.ArtistInfoTwo = &spec.ArtistInfo{}
	if artist.Cover != "" {
		sub.ArtistInfoTwo.SmallImageURL = c.genArtistCoverURL(r, &artist, 64)
		sub.ArtistInfoTwo.MediumImageURL = c.genArtistCoverURL(r, &artist, 126)
		sub.ArtistInfoTwo.LargeImageURL = c.genArtistCoverURL(r, &artist, 256)
	}

	apiKey, _ := c.DB.GetSetting("lastfm_api_key")
	if apiKey == "" {
		return sub
	}
	info, err := c.LastFMClient.ArtistGetInfo(apiKey, artist.Name)
	if err != nil {
		return spec.NewError(0, "fetching artist info: %v", err)
	}

	sub.ArtistInfoTwo.Biography = info.Bio.Summary
	sub.ArtistInfoTwo.MusicBrainzID = info.MBID
	sub.ArtistInfoTwo.LastFMURL = info.URL

	if artist.Cover == "" {
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
		if url, _ := c.LastFMClient.StealArtistImage(info.URL); url != "" {
			sub.ArtistInfoTwo.SmallImageURL = url
			sub.ArtistInfoTwo.MediumImageURL = url
			sub.ArtistInfoTwo.LargeImageURL = url
			sub.ArtistInfoTwo.ArtistImageURL = url
		}
	}

	count := params.GetOrInt("count", 20)
	inclNotPresent := params.GetOrBool("includeNotPresent", false)
	similarArtists, err := c.LastFMClient.ArtistGetSimilar(apiKey, artist.Name)
	if err != nil {
		return spec.NewError(0, "fetching artist similar: %v", err)
	}

	for i, similarInfo := range similarArtists.Artists {
		if i == count {
			break
		}
		var artist db.Artist
		err = c.DB.
			Select("artists.*, count(albums.id) album_count").
			Where("name=?", similarInfo.Name).
			Joins("LEFT JOIN albums ON artists.id=albums.tag_artist_id").
			Group("artists.id").
			Find(&artist).
			Error
		if errors.Is(err, gorm.ErrRecordNotFound) && !inclNotPresent {
			continue
		}
		artistID := &specid.ID{}
		if artist.ID != 0 {
			artistID = artist.SID()
		}
		sub.ArtistInfoTwo.SimilarArtist = append(sub.ArtistInfoTwo.SimilarArtist, &spec.SimilarArtist{
			ID:         artistID,
			Name:       similarInfo.Name,
			AlbumCount: artist.AlbumCount,
		})
	}

	return sub
}

func (c *Controller) ServeGetGenres(_ *http.Request) *spec.Response {
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
	user := r.Context().Value(CtxUser).(*db.User)
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
		Preload("Album.TagArtist").
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID).
		Offset(params.GetOrInt("offset", 0)).
		Limit(params.GetOrInt("count", 10))
	if m := getMusicFolder(c.MusicPaths, params); m != "" {
		q = q.Where("albums.root_dir=?", m)
	}
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "error finding tracks: %v", err)
	}
	sub := spec.NewResponse()
	sub.TracksByGenre = &spec.TracksByGenre{
		List: make([]*spec.TrackChild, len(tracks)),
	}

	transcodeMIME, transcodeSuffix := streamGetTransPrefProfile(c.DB, user.ID, params.GetOr("c", ""))

	for i, t := range tracks {
		sub.TracksByGenre.List[i] = spec.NewTrackByTags(t, t.Album)
		sub.TracksByGenre.List[i].TranscodedContentType = transcodeMIME
		sub.TracksByGenre.List[i].TranscodedSuffix = transcodeSuffix
	}

	return sub
}

func (c *Controller) ServeGetStarredTwo(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(params.Params)

	results := &spec.StarredTwo{}

	// artists
	var artists []*db.Artist
	q := c.DB.
		Group("artists.id").
		Joins("JOIN artist_stars ON artist_stars.artist_id=artists.id").
		Where("artist_stars.user_id=?", user.ID).
		Preload("ArtistStar", "user_id=?", user.ID).
		Preload("ArtistRating", "user_id=?", user.ID)
	if m := getMusicFolder(c.MusicPaths, params); m != "" {
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
	q = c.DB.
		Joins("JOIN album_stars ON album_stars.album_id=albums.id").
		Where("album_stars.user_id=?", user.ID).
		Preload("TagArtist").
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID)
	if m := getMusicFolder(c.MusicPaths, params); m != "" {
		q = q.Where("albums.root_dir=?", m)
	}
	if err := q.Find(&albums).Error; err != nil {
		return spec.NewError(0, "find albums: %v", err)
	}
	for _, a := range albums {
		results.Albums = append(results.Albums, spec.NewAlbumByTags(a, a.TagArtist))
	}

	// tracks
	var tracks []*db.Track
	q = c.DB.
		Joins("JOIN track_stars ON tracks.id=track_stars.track_id").
		Where("track_stars.user_id=?", user.ID).
		Preload("Album").
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID)
	if m := getMusicFolder(c.MusicPaths, params); m != "" {
		q = q.
			Joins("JOIN albums ON albums.id=tracks.album_id").
			Where("albums.root_dir=?", m)
	}
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find tracks: %v", err)
	}

	transcodeMIME, transcodeSuffix := streamGetTransPrefProfile(c.DB, user.ID, params.GetOr("c", ""))

	for _, t := range tracks {
		track := spec.NewTrackByTags(t, t.Album)
		track.TranscodedContentType = transcodeMIME
		track.TranscodedSuffix = transcodeSuffix
		results.Tracks = append(results.Tracks, track)
	}

	sub := spec.NewResponse()
	sub.StarredTwo = results
	return sub
}

func (c *Controller) genArtistCoverURL(r *http.Request, artist *db.Artist, size int) string {
	coverURL, _ := url.Parse(c.BaseURL(r))
	coverURL.Path = c.Path("/rest/getCoverArt")

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
	if err := c.DB.Where("name=?", artistName).Find(&artist).Error; err != nil {
		return spec.NewError(0, "finding artist by name: %v", err)
	}

	apiKey, _ := c.DB.GetSetting("lastfm_api_key")
	if apiKey == "" {
		return spec.NewResponse()
	}
	topTracks, err := c.LastFMClient.ArtistGetTopTracks(apiKey, artist.Name)
	if err != nil {
		return spec.NewError(0, "fetching artist top tracks: %v", err)
	}

	sub := spec.NewResponse()
	sub.TopSongs = &spec.TopSongs{
		Tracks: make([]*spec.TrackChild, 0),
	}

	if len(topTracks.Tracks) == 0 {
		return sub
	}

	topTrackNames := make([]string, len(topTracks.Tracks))
	for i, t := range topTracks.Tracks {
		topTrackNames[i] = t.Name
	}

	var tracks []*db.Track
	err = c.DB.
		Preload("Album").
		Where("artist_id=? AND tracks.tag_title IN (?)", artist.ID, topTrackNames).
		Limit(count).
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID).
		Find(&tracks).
		Error
	if err != nil {
		return spec.NewError(0, "error finding tracks: %v", err)
	}
	if len(tracks) == 0 {
		return sub
	}

	transcodeMIME, transcodeSuffix := streamGetTransPrefProfile(c.DB, user.ID, params.GetOr("c", ""))

	for _, track := range tracks {
		tc := spec.NewTrackByTags(track, track.Album)
		tc.TranscodedContentType = transcodeMIME
		tc.TranscodedSuffix = transcodeSuffix
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
	apiKey, _ := c.DB.GetSetting("lastfm_api_key")
	if apiKey == "" {
		return spec.NewResponse()
	}

	var track db.Track
	err = c.DB.
		Preload("Artist").
		Preload("Album").
		Where("id=?", id.Value).
		First(&track).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(10, "couldn't find a track with that id")
	}

	similarTracks, err := c.LastFMClient.TrackGetSimilarTracks(apiKey, track.Artist.Name, track.TagTitle)
	if err != nil {
		return spec.NewError(0, "fetching track similar tracks: %v", err)
	}
	if len(similarTracks.Tracks) == 0 {
		return spec.NewError(70, "no similar songs found for track: %v", track.TagTitle)
	}

	similarTrackNames := make([]string, len(similarTracks.Tracks))
	for i, t := range similarTracks.Tracks {
		similarTrackNames[i] = t.Name
	}

	var tracks []*db.Track
	err = c.DB.
		Preload("Artist").
		Preload("Album").
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID).
		Select("tracks.*").
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

	transcodeMIME, transcodeSuffix := streamGetTransPrefProfile(c.DB, user.ID, params.GetOr("c", ""))

	for i, track := range tracks {
		sub.SimilarSongs.Tracks[i] = spec.NewTrackByTags(track, track.Album)
		sub.SimilarSongs.Tracks[i].TranscodedContentType = transcodeMIME
		sub.SimilarSongs.Tracks[i].TranscodedSuffix = transcodeSuffix
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

	apiKey, _ := c.DB.GetSetting("lastfm_api_key")
	if apiKey == "" {
		return spec.NewResponse()
	}

	var artist db.Artist
	err = c.DB.
		Where("id=?", id.Value).
		First(&artist).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(0, "artist with id `%s` not found", id)
	}

	similarArtists, err := c.LastFMClient.ArtistGetSimilar(apiKey, artist.Name)
	if err != nil {
		return spec.NewError(0, "fetching artist similar artists: %v", err)
	}
	if len(similarArtists.Artists) == 0 {
		return spec.NewError(0, "no similar artist found for: %v", artist.Name)
	}

	artistNames := make([]string, len(similarArtists.Artists))
	for i, similarArtist := range similarArtists.Artists {
		artistNames[i] = similarArtist.Name
	}

	var tracks []*db.Track
	err = c.DB.
		Preload("Album").
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID).
		Joins("JOIN artists on tracks.artist_id=artists.id").
		Where("artists.name IN (?)", artistNames).
		Order(gorm.Expr("random()")).
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

	transcodeMIME, transcodeSuffix := streamGetTransPrefProfile(c.DB, user.ID, params.GetOr("c", ""))

	for i, track := range tracks {
		sub.SimilarSongsTwo.Tracks[i] = spec.NewTrackByTags(track, track.Album)
		sub.SimilarSongsTwo.Tracks[i].TranscodedContentType = transcodeMIME
		sub.SimilarSongsTwo.Tracks[i].TranscodedSuffix = transcodeSuffix
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
		_ = c.DB.Where("user_id=? AND album_id=?", user.ID, id).First(&albumstar).Error
		albumstar.UserID = user.ID
		albumstar.AlbumID = id
		albumstar.StarDate = stardate
		if err := c.DB.Save(&albumstar).Error; err != nil {
			return spec.NewError(0, "save album star: %v", err)
		}
	}

	for _, id := range starIDsOfType(params, specid.Artist) {
		var artiststar db.ArtistStar
		_ = c.DB.Where("user_id=? AND artist_id=?", user.ID, id).First(&artiststar).Error
		artiststar.UserID = user.ID
		artiststar.ArtistID = id
		artiststar.StarDate = stardate
		if err := c.DB.Save(&artiststar).Error; err != nil {
			return spec.NewError(0, "save artist star: %v", err)
		}
	}

	for _, id := range starIDsOfType(params, specid.Track) {
		var trackstar db.TrackStar
		_ = c.DB.Where("user_id=? AND track_id=?", user.ID, id).First(&trackstar).Error
		trackstar.UserID = user.ID
		trackstar.TrackID = id
		trackstar.StarDate = stardate
		if err := c.DB.Save(&trackstar).Error; err != nil {
			return spec.NewError(0, "save track star: %v", err)
		}
	}

	return spec.NewResponse()
}

func (c *Controller) ServeUnstar(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)

	for _, id := range starIDsOfType(params, specid.Album) {
		if err := c.DB.Where("user_id=? AND album_id=?", user.ID, id).Delete(db.AlbumStar{}).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(0, "delete album star: %v", err)
		}
	}

	for _, id := range starIDsOfType(params, specid.Artist) {
		if err := c.DB.Where("user_id=? AND artist_id=?", user.ID, id).Delete(db.ArtistStar{}).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(0, "delete artist star: %v", err)
		}
	}

	for _, id := range starIDsOfType(params, specid.Track) {
		if err := c.DB.Where("user_id=? AND track_id=?", user.ID, id).Delete(db.TrackStar{}).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
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
		err := c.DB.Where("id=?", id.Value).First(&album).Error
		if err != nil {
			return spec.NewError(0, "fetch album: %v", err)
		}
		var albumRating db.AlbumRating
		if err := c.DB.Where("user_id=? AND album_id=?", user.ID, id.Value).First(&albumRating).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(0, "fetch album rating: %v", err)
		}
		switch {
		case rating == 0 && albumRating.AlbumID == album.ID:
			if err := c.DB.Delete(&albumRating).Error; err != nil {
				return spec.NewError(0, "delete album rating: %v", err)
			}
		case rating > 0:
			albumRating.UserID = user.ID
			albumRating.AlbumID = id.Value
			albumRating.Rating = rating
			if err := c.DB.Save(&albumRating).Error; err != nil {
				return spec.NewError(0, "save album rating: %v", err)
			}
		}
		var averageRating float64
		if err := c.DB.Model(db.AlbumRating{}).Select("coalesce(avg(rating), 0)").Where("album_id=?", id.Value).Row().Scan(&averageRating); err != nil {
			return spec.NewError(0, "find average album rating: %v", err)
		}
		album.AverageRating = math.Trunc(averageRating*100) / 100
		if err := c.DB.Save(&album).Error; err != nil {
			return spec.NewError(0, "save album: %v", err)
		}
	case specid.Artist:
		var artist db.Artist
		err := c.DB.Where("id=?", id.Value).First(&artist).Error
		if err != nil {
			return spec.NewError(0, "fetch artist: %v", err)
		}
		var artistRating db.ArtistRating
		if err := c.DB.Where("user_id=? AND artist_id=?", user.ID, id.Value).First(&artistRating).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(0, "fetch artist rating: %v", err)
		}
		switch {
		case rating == 0 && artistRating.ArtistID == artist.ID:
			if err := c.DB.Delete(&artistRating).Error; err != nil {
				return spec.NewError(0, "delete artist rating: %v", err)
			}
		case rating > 0:
			artistRating.UserID = user.ID
			artistRating.ArtistID = id.Value
			artistRating.Rating = rating
			if err := c.DB.Save(&artistRating).Error; err != nil {
				return spec.NewError(0, "save artist rating: %v", err)
			}
		}
		var averageRating float64
		if err := c.DB.Model(db.ArtistRating{}).Select("coalesce(avg(rating), 0)").Where("artist_id=?", id.Value).Row().Scan(&averageRating); err != nil {
			return spec.NewError(0, "find average artist rating: %v", err)
		}
		artist.AverageRating = math.Trunc(averageRating*100) / 100
		if err := c.DB.Save(&artist).Error; err != nil {
			return spec.NewError(0, "save artist: %v", err)
		}
	case specid.Track:
		var track db.Track
		err := c.DB.Where("id=?", id.Value).First(&track).Error
		if err != nil {
			return spec.NewError(0, "fetch track: %v", err)
		}
		var trackRating db.TrackRating
		if err := c.DB.Where("user_id=? AND track_id=?", user.ID, id.Value).First(&trackRating).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(0, "fetch track rating: %v", err)
		}
		switch {
		case rating == 0 && trackRating.TrackID == track.ID:
			if err := c.DB.Delete(&trackRating).Error; err != nil {
				return spec.NewError(0, "delete track rating: %v", err)
			}
		case rating > 0:
			trackRating.UserID = user.ID
			trackRating.TrackID = id.Value
			trackRating.Rating = rating
			if err := c.DB.Save(&trackRating).Error; err != nil {
				return spec.NewError(0, "save track rating: %v", err)
			}
		}
		var averageRating float64
		if err := c.DB.Model(db.TrackRating{}).Select("coalesce(avg(rating), 0)").Where("track_id=?", id.Value).Row().Scan(&averageRating); err != nil {
			return spec.NewError(0, "find average track rating: %v", err)
		}
		track.AverageRating = math.Trunc(averageRating*100) / 100
		if err := c.DB.Save(&track).Error; err != nil {
			return spec.NewError(0, "save track: %v", err)
		}
	default:
		return spec.NewError(0, "non-album non-artist non-track id cannot be rated")
	}

	return spec.NewResponse()
}
