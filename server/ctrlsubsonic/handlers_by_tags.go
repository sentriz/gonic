//nolint:goconst
package ctrlsubsonic

import (
	"errors"
	"log"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/handlerutil"
	"go.senan.xyz/gonic/infocache/albuminfocache"
	"go.senan.xyz/gonic/infocache/artistinfocache"
	"go.senan.xyz/gonic/lastfm"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

func (c *Controller) ServeGetArtists(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	var artists []*spec.ArtistRow
	q := c.dbc.
		Scopes(spec.LoadArtistByTags(user.ID)).
		Joins("JOIN album_credits ON album_credits.artist_id=artists.id AND album_credits.role=?", db.RoleAlbumArtist).
		Order("artists.name COLLATE NOCASE")
	if m := getMusicFolder(c.musicPaths, params); m != "" {
		q = q.
			Joins("JOIN albums ON albums.id=album_credits.album_id").
			Scopes(spec.WithAlbumRootDir(m))
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
	var artist spec.ArtistRow
	if err := c.dbc.
		Scopes(spec.ArtistWithRoles, spec.ArtistWithUserData(user.ID)).
		Preload("Info").
		First(&artist, id.Value).
		Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(70, "couldn't find an artist with that id")
		}
		return spec.NewError(0, "find artist: %v", err)
	}

	var appearances []*spec.AlbumRow
	if err := c.dbc.
		Scopes(spec.LoadAlbumByTags(user.ID)).
		Where(`albums.id IN (
			SELECT album_id FROM album_credits WHERE artist_id=?
			UNION
			SELECT tracks.album_id FROM track_credits
				JOIN tracks ON tracks.id=track_credits.track_id
				WHERE track_credits.artist_id=?
		)`, artist.ID, artist.ID).
		Order(c.sortRightPathPrefixed()).
		Find(&appearances).Error; err != nil {
		return spec.NewError(0, "find artist appearances: %v", err)
	}

	sub := spec.NewResponse()
	sub.Artist = spec.NewArtistByTags(&artist)
	sub.Artist.Albums = make([]*spec.Album, len(appearances))
	for i, album := range appearances {
		sub.Artist.Albums[i] = spec.NewAlbumByTags(album, album.Credits)
	}
	sub.Artist.AlbumCount = len(appearances)
	return sub
}

func (c *Controller) ServeGetAlbum(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	album := &spec.AlbumRow{}
	err = c.dbc.
		Scopes(spec.LoadAlbumByTags(user.ID)).
		First(album, id.Value).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(70, "couldn't find an album with that id")
	}
	if err != nil {
		return spec.NewError(0, "find album: %v", err)
	}

	var tracks []*spec.TrackRow
	if err := c.dbc.
		Scopes(spec.LoadTrackByTags(user.ID)).
		Where("album_id=?", id.Value).
		Order("tracks.tag_disc_number, tracks.tag_track_number").
		Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find album tracks: %v", err)
	}

	sub := spec.NewResponse()
	sub.Album = spec.NewAlbumByTags(album, album.Credits)
	sub.Album.Tracks = make([]*spec.TrackChild, len(tracks))

	client := params.GetOr("c", "")
	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, client)

	for i, track := range tracks {
		sub.Album.Tracks[i] = spec.NewTrackByTags(client, track, &album.Album)
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
		q = q.Joins("JOIN artists ON artists.id=album_credits.artist_id")
		q = q.Order("artists.name")
	case "alphabeticalByName":
		q = q.Order("albums.tag_title")
	case "byYear":
		y1, y2 := params.GetOrInt("fromYear", 1800),
			params.GetOrInt("toYear", 2200)
		// support some clients sending wrong order like DSub
		q = q.Where("albums.tag_year BETWEEN ? AND ?", min(y1, y2), max(y1, y2))
		q = q.Order("albums.tag_year DESC")
	case "byGenre":
		genre, _ := params.Get("genre")
		q = q.Joins("JOIN album_genres ON album_genres.album_id=albums.id")
		q = q.Joins("JOIN genres ON genres.id=album_genres.genre_id AND genres.name=?", genre)
		q = q.Order("albums.tag_title")
	case "frequent":
		q = q.Having("play_length > 0").Order("play_length DESC")
	case "newest":
		q = q.Order("albums.created_at DESC")
	case "random":
		q = q.Order(gorm.Expr("random()"))
	case "recent":
		q = q.Having("play_time IS NOT NULL").Order("play_time DESC")
	case "starred":
		q = q.Joins("JOIN album_stars ON albums.id=album_stars.album_id AND album_stars.user_id=?", user.ID)
		q = q.Order("albums.tag_title")
	case "highest":
		q = q.Joins("JOIN album_ratings ON album_ratings.album_id=albums.id AND album_ratings.user_id=?", user.ID)
		q = q.Order("album_ratings.rating DESC")
	default:
		return spec.NewError(10, "unknown value %q for parameter 'type'", listType)
	}
	q = q.Scopes(spec.WithAlbumRootDir(getMusicFolder(c.musicPaths, params)))
	var albums []*spec.AlbumRow
	// TODO: think about removing this extra join to count number
	// of children. it might make sense to store that in the db
	err = q.
		Scopes(spec.LoadAlbumByTags(user.ID)).
		Joins("JOIN album_credits ON album_credits.album_id=albums.id AND album_credits.role=?", db.RoleAlbumArtist).
		Offset(params.GetOrInt("offset", 0)).
		Limit(params.GetOrInt("size", 10)).
		Find(&albums).
		Error
	if err != nil {
		return spec.NewError(0, "error finding albums: %v", err)
	}
	sub := spec.NewResponse()
	sub.AlbumsTwo = &spec.Albums{
		List: make([]*spec.Album, len(albums)),
	}
	for i, album := range albums {
		sub.AlbumsTwo.List[i] = spec.NewAlbumByTags(album, album.Credits)
	}
	return sub
}

func (c *Controller) ServeSearchThree(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	query, err := params.Get("query")
	if err != nil {
		return spec.NewError(10, "please provide a `query` parameter")
	}

	var isUUID = uuid.Validate(query) == nil
	var isAll = query == `""`

	var fuzzy = query
	fuzzy = strings.Join(strings.Fields(fuzzy), "%")
	fuzzy = strings.ToLower(fuzzy)
	fuzzy = "%" + fuzzy + "%"

	results := &spec.SearchResultThree{}

	musicFolder := getMusicFolder(c.musicPaths, params)

	// search artists
	var artists []*spec.ArtistRow
	q := c.dbc.
		Scopes(spec.LoadArtistByTags(user.ID))
	switch {
	case isUUID:
		q = q.Where(0)
	case isAll:
	default:
		q = q.Where(`name LIKE ? OR name_u_dec LIKE ?`, fuzzy, fuzzy)
	}
	q = q.
		Joins("JOIN album_credits ON album_credits.artist_id=artists.id AND album_credits.role=?", db.RoleAlbumArtist).
		Joins("JOIN albums ON albums.id=album_credits.album_id").
		Scopes(spec.WithAlbumRootDir(musicFolder)).
		Offset(params.GetOrInt("artistOffset", 0)).
		Limit(params.GetOrInt("artistCount", 20))
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(0, "find artists: %v", err)
	}
	for _, a := range artists {
		results.Artists = append(results.Artists, spec.NewArtistByTags(a))
	}

	// search albums
	var albums []*spec.AlbumRow
	q = c.dbc.
		Scopes(spec.LoadAlbumByTags(user.ID), spec.WithAlbumRootDir(musicFolder))
	switch {
	case isUUID:
		q = q.Where(`albums.tag_brainz_id = ?`, query)
	case isAll:
	default:
		q = q.Where(`albums.tag_title LIKE ? OR albums.tag_title_u_dec LIKE ?`, fuzzy, fuzzy)
	}
	q = q.
		Offset(params.GetOrInt("albumOffset", 0)).
		Limit(params.GetOrInt("albumCount", 20))
	if err := q.Find(&albums).Error; err != nil {
		return spec.NewError(0, "find albums: %v", err)
	}
	for _, a := range albums {
		results.Albums = append(results.Albums, spec.NewAlbumByTags(a, a.Credits))
	}

	// search tracks
	var tracks []*spec.TrackRow
	q = c.dbc.
		Scopes(spec.LoadTrackByTags(user.ID))
	switch {
	case isUUID:
		q = q.Where(`tracks.tag_brainz_id = ?`, query)
	case isAll:
	default:
		q = q.Where(`tracks.tag_title LIKE ? OR tracks.tag_title_u_dec LIKE ?`, fuzzy, fuzzy)
	}
	q = q.Offset(params.GetOrInt("songOffset", 0)).
		Limit(params.GetOrInt("songCount", 20))
	if musicFolder != "" {
		q = q.
			Joins("JOIN albums ON albums.id=tracks.album_id").
			Scopes(spec.WithAlbumRootDir(musicFolder))
	}
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find tracks: %v", err)
	}

	client := params.GetOr("c", "")
	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, client)

	for _, t := range tracks {
		track := spec.NewTrackByTags(client, t, t.Album)
		track.TranscodeMeta = transcodeMeta
		results.Tracks = append(results.Tracks, track)
	}

	sub := spec.NewResponse()
	sub.SearchResultThree = results
	return sub
}

func (c *Controller) ServeGetArtistInfoTwo(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
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
	if err != nil {
		return spec.NewError(0, "find artist: %v", err)
	}

	sub := spec.NewResponse()
	sub.ArtistInfoTwo = &spec.ArtistInfo{}

	info, err := c.artistInfoCache.GetOrLookup(r.Context(), artist.ID)
	if err != nil {
		log.Printf("error fetching artist info from lastfm: %v", err)
		return sub
	}

	sub.ArtistInfoTwo.Biography = artistinfocache.Biography(info)
	sub.ArtistInfoTwo.MusicBrainzID = info.MusicBrainzID
	sub.ArtistInfoTwo.LastFMURL = info.LastFMURL

	if err := uuid.Validate(artist.MusicBrainzID); err == nil {
		sub.ArtistInfoTwo.MusicBrainzID = artist.MusicBrainzID // prefer db musicbrainz ID over lastfm's
	}

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

	var similar []string
	seen := map[string]struct{}{}
	for _, name := range slices.Concat(info.GetLastFMSimilarArtists(), info.GetMusicBrainzRelatedArtists()) {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		similar = append(similar, name)
	}

	for i, similarName := range similar {
		if i == count {
			break
		}
		var artist spec.ArtistRow
		err = c.dbc.
			Scopes(spec.LoadArtistByTags(user.ID)).
			Where("name=?", similarName).
			Joins("LEFT JOIN album_credits ON album_credits.artist_id=artists.id AND album_credits.role=?", db.RoleAlbumArtist).
			Joins("LEFT JOIN albums ON albums.id=album_credits.album_id").
			Find(&artist).
			Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(0, "find similar artist: %v", err)
		}

		if artist.ID == 0 {
			if !inclNotPresent {
				continue
			}
			// add a very limited artist, since we don't have everything with `inclNotPresent`
			sub.ArtistInfoTwo.Similar = append(sub.ArtistInfoTwo.Similar, &spec.Artist{
				ID:    &specid.ID{},
				Name:  similarName,
				Roles: []string{},
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
	if err != nil {
		return spec.NewError(0, "find album: %v", err)
	}

	sub := spec.NewResponse()
	sub.AlbumInfo = &spec.AlbumInfo{}

	info, err := c.albumInfoCache.GetOrLookup(r.Context(), album.ID)
	if err != nil {
		log.Printf("error fetching album info from lastfm: %v", err)
		return sub
	}

	sub.AlbumInfo.Notes = albuminfocache.Notes(info)
	sub.AlbumInfo.MusicBrainzID = info.MusicBrainzID
	sub.AlbumInfo.LastFMURL = info.LastFMURL

	if err := uuid.Validate(album.TagBrainzID); err == nil {
		sub.AlbumInfo.MusicBrainzID = album.TagBrainzID // prefer db musicbrainz ID over lastfm's
	}

	return sub
}

func (c *Controller) ServeGetGenres(_ *http.Request) *spec.Response {
	var genres []*spec.GenreRow
	err := c.dbc.
		Scopes(spec.GenreWithCounts).
		Order("genres.name").
		Find(&genres).
		Error
	if err != nil {
		return spec.NewError(0, "error finding genres: %v", err)
	}
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
	var tracks []*spec.TrackRow
	q := c.dbc.
		Scopes(spec.LoadTrackByTags(user.ID), spec.WithAlbumRootDir(getMusicFolder(c.musicPaths, params))).
		Joins("JOIN albums ON tracks.album_id=albums.id").
		Joins("JOIN track_genres ON track_genres.track_id=tracks.id").
		Joins("JOIN genres ON track_genres.genre_id=genres.id AND genres.name=?", genre).
		Offset(params.GetOrInt("offset", 0)).
		Limit(params.GetOrInt("count", 10)).
		Group("tracks.id")
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "error finding tracks: %v", err)
	}
	sub := spec.NewResponse()
	sub.TracksByGenre = &spec.TracksByGenre{
		List: make([]*spec.TrackChild, len(tracks)),
	}

	client := params.GetOr("c", "")
	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, client)

	for i, t := range tracks {
		sub.TracksByGenre.List[i] = spec.NewTrackByTags(client, t, t.Album)
		sub.TracksByGenre.List[i].TranscodeMeta = transcodeMeta
	}

	return sub
}

func (c *Controller) ServeGetStarredTwo(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	params := r.Context().Value(CtxParams).(params.Params)
	musicFolder := getMusicFolder(c.musicPaths, params)

	results := &spec.StarredTwo{}

	// artists
	var artists []*spec.ArtistRow
	q := c.dbc.
		Scopes(spec.LoadArtistByTags(user.ID), spec.WithAlbumRootDir(musicFolder)).
		Joins("JOIN artist_stars ON artist_stars.artist_id=artists.id").
		Where("artist_stars.user_id=?", user.ID).
		Joins("JOIN album_credits ON album_credits.artist_id=artists.id AND album_credits.role=?", db.RoleAlbumArtist).
		Joins("JOIN albums ON albums.id=album_credits.album_id").
		Order("artist_stars.star_date DESC")
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(0, "find artists: %v", err)
	}
	for _, a := range artists {
		results.Artists = append(results.Artists, spec.NewArtistByTags(a))
	}

	// albums
	var albums []*spec.AlbumRow
	q = c.dbc.
		Scopes(spec.LoadAlbumByTags(user.ID), spec.WithAlbumRootDir(musicFolder)).
		Joins("JOIN album_stars ON album_stars.album_id=albums.id").
		Where("album_stars.user_id=?", user.ID).
		Order("album_stars.star_date DESC")
	if err := q.Find(&albums).Error; err != nil {
		return spec.NewError(0, "find albums: %v", err)
	}
	for _, a := range albums {
		results.Albums = append(results.Albums, spec.NewAlbumByTags(a, a.Credits))
	}

	// tracks
	var tracks []*spec.TrackRow
	q = c.dbc.
		Scopes(spec.LoadTrackByTags(user.ID)).
		Joins("JOIN track_stars ON tracks.id=track_stars.track_id").
		Where("track_stars.user_id=?", user.ID).
		Order("track_stars.star_date DESC")
	if musicFolder != "" {
		q = q.
			Joins("JOIN albums ON albums.id=tracks.album_id").
			Scopes(spec.WithAlbumRootDir(musicFolder))
	}
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find tracks: %v", err)
	}

	client := params.GetOr("c", "")
	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, client)

	for _, t := range tracks {
		track := spec.NewTrackByTags(client, t, t.Album)
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

	topTrackNames := info.GetLastFMTopTracks()
	if len(topTrackNames) == 0 {
		return sub
	}

	var tracks []*spec.TrackRow
	err = c.dbc.
		Scopes(spec.LoadTrackByTags(user.ID)).
		Where("tracks.tag_title IN (?)", topTrackNames).
		Joins("JOIN track_credits ON track_credits.track_id=tracks.id AND track_credits.role=?", db.RoleArtist).
		Joins("JOIN artists ON artists.id=track_credits.artist_id").
		Where("artists.id=?", artist.ID).
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

	client := params.GetOr("c", "")
	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, client)

	for _, track := range tracks {
		tc := spec.NewTrackByTags(client, track, track.Album)
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
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}

	var tracks []*spec.TrackChild
	var sub *spec.Response

	switch id.Type {
	case specid.Track:
		tracks, sub = getSimilarSongsFromTrack(c, id, params, user, count)
	case specid.Album:
		tracks, sub = getSimilarSongsFromAlbum(c, id, params, user, count)
	case specid.Artist:
		tracks, sub = getSimilarSongsFromArtist(c, id, params, user, count)
	default:
		return spec.NewError(10, "please provide a artist, album or track `id` parameter")
	}

	if sub != nil {
		return sub
	}

	sub = spec.NewResponse()
	sub.SimilarSongs = &spec.SimilarSongs{
		Tracks: tracks,
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

	tracks, sub := getSimilarSongsFromArtist(c, id, params, user, count)
	if sub != nil {
		return sub
	}

	sub = spec.NewResponse()
	sub.SimilarSongsTwo = &spec.SimilarSongsTwo{
		Tracks: tracks,
	}
	return sub
}

func getSimilarSongsFromTrack(c *Controller, id specid.ID, params params.Params, user *db.User, count int) ([]*spec.TrackChild, *spec.Response) {
	var track db.Track
	err := c.dbc.
		Preload("Album").
		Where("id=?", id.Value).
		First(&track).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, spec.NewError(70, "couldn't find a track with that id")
	}
	if err != nil {
		return nil, spec.NewError(0, "find track: %v", err)
	}

	similarTracks, err := c.lastFMClient.TrackGetSimilarTracks(track.TagTrackArtist, track.TagTitle)
	if err != nil {
		log.Printf("error fetching similar songs from lastfm: %v", err)
		return nil, spec.NewResponse()
	}

	if len(similarTracks.Tracks) == 0 {
		return nil, spec.NewError(70, "no similar songs found for track: %v", track.TagTitle)
	}

	similarTrackNames := make([]string, len(similarTracks.Tracks))
	for i, t := range similarTracks.Tracks {
		similarTrackNames[i] = t.Name
	}

	var tracks []*spec.TrackRow
	err = c.dbc.
		Scopes(spec.LoadTrackByTags(user.ID)).
		Where("tracks.tag_title IN (?)", similarTrackNames).
		Order(gorm.Expr("random()")).
		Limit(count).
		Find(&tracks).
		Error
	if err != nil {
		return nil, spec.NewError(0, "error finding tracks: %v", err)
	}
	if len(tracks) == 0 {
		return nil, spec.NewError(70, "no similar song could be match with collection in database: %v", track.TagTitle)
	}

	trackChildren := make([]*spec.TrackChild, len(tracks))
	client := params.GetOr("c", "")
	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, client)

	for i, track := range tracks {
		trackChildren[i] = spec.NewTrackByTags(client, track, track.Album)
		trackChildren[i].TranscodeMeta = transcodeMeta
	}

	return trackChildren, nil
}

func getSimilarSongsFromArtist(c *Controller, id specid.ID, params params.Params, user *db.User, count int) ([]*spec.TrackChild, *spec.Response) {
	var artist db.Artist
	err := c.dbc.
		Where("id=?", id.Value).
		First(&artist).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, spec.NewError(70, "artist with id %q not found", id)
	}
	if err != nil {
		return nil, spec.NewError(0, "find artist: %v", err)
	}

	similarArtists, err := c.lastFMClient.ArtistGetSimilar(artist.Name)
	if err != nil {
		log.Printf("error fetching artist info from lastfm: %v", err)
		return nil, spec.NewResponse()
	}
	if len(similarArtists.Artists) == 0 {
		return nil, spec.NewError(0, "no similar artist found for: %v", artist.Name)
	}

	artistNames := make([]string, len(similarArtists.Artists))
	for i, similarArtist := range similarArtists.Artists {
		artistNames[i] = similarArtist.Name
	}

	var tracks []*spec.TrackRow
	err = c.dbc.
		Scopes(spec.LoadTrackByTags(user.ID)).
		Joins("JOIN track_credits ON track_credits.track_id=tracks.id AND track_credits.role=?", db.RoleArtist).
		Joins("JOIN artists ON artists.id=track_credits.artist_id").
		Where("artists.name IN (?)", artistNames).
		Order(gorm.Expr("random()")).
		Group("tracks.id").
		Limit(count).
		Find(&tracks).
		Error
	if err != nil {
		return nil, spec.NewError(0, "error finding tracks: %v", err)
	}
	if len(tracks) == 0 {
		return nil, spec.NewError(70, "no similar song could be match with collection in database: %v", artist.Name)
	}

	trackChildren := make([]*spec.TrackChild, len(tracks))
	client := params.GetOr("c", "")
	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, client)

	for i, track := range tracks {
		trackChildren[i] = spec.NewTrackByTags(client, track, track.Album)
		trackChildren[i].TranscodeMeta = transcodeMeta
	}

	return trackChildren, nil
}

func getSimilarSongsFromAlbum(c *Controller, id specid.ID, params params.Params, user *db.User, count int) ([]*spec.TrackChild, *spec.Response) {
	var album db.Album
	err := c.dbc.
		Preload("Tracks").
		Where("id=?", id.Value).
		First(&album).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, spec.NewError(70, "couldn't find an album with that id")
	}
	if err != nil {
		return nil, spec.NewError(0, "find album: %v", err)
	}

	var similarTracks lastfm.SimilarTracks

	for _, albumTrack := range album.Tracks {
		similarTracks, err = c.lastFMClient.TrackGetSimilarTracks(albumTrack.TagTrackArtist, albumTrack.TagTitle)
		if err != nil {
			log.Printf("error fetching similar songs from lastfm: %v", err)
			continue
		}
		if len(similarTracks.Tracks) == 0 {
			log.Printf("no similar songs found for track: %v", albumTrack.TagTitle)
			continue
		}
		break
	}

	if len(similarTracks.Tracks) == 0 {
		return nil, spec.NewError(0, "no similar songs found for album: %v", album.TagTitle)
	}

	similarTrackNames := make([]string, len(similarTracks.Tracks))
	for i, t := range similarTracks.Tracks {
		similarTrackNames[i] = t.Name
	}

	var tracks []*spec.TrackRow
	err = c.dbc.
		Scopes(spec.LoadTrackByTags(user.ID)).
		Where("tracks.tag_title IN (?)", similarTrackNames).
		Order(gorm.Expr("random()")).
		Limit(count).
		Find(&tracks).
		Error
	if err != nil {
		return nil, spec.NewError(0, "error finding tracks: %v", err)
	}
	if len(tracks) == 0 {
		return nil, spec.NewError(70, "no similar songs could be matched with collection in database: %v", album.TagTitle)
	}

	trackChildren := make([]*spec.TrackChild, len(tracks))
	client := params.GetOr("c", "")
	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, client)

	for i, track := range tracks {
		trackChildren[i] = spec.NewTrackByTags(client, track, track.Album)
		trackChildren[i].TranscodeMeta = transcodeMeta
	}

	return trackChildren, nil
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
		if err := c.dbc.Where("user_id=? AND album_id=?", user.ID, id).First(&albumstar).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(0, "find album star: %v", err)
		}
		albumstar.UserID = user.ID
		albumstar.AlbumID = id
		albumstar.StarDate = stardate
		if err := c.dbc.Save(&albumstar).Error; err != nil {
			return spec.NewError(0, "save album star: %v", err)
		}
	}

	for _, id := range starIDsOfType(params, specid.Artist) {
		var artiststar db.ArtistStar
		if err := c.dbc.Where("user_id=? AND artist_id=?", user.ID, id).First(&artiststar).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(0, "find artist star: %v", err)
		}
		artiststar.UserID = user.ID
		artiststar.ArtistID = id
		artiststar.StarDate = stardate
		if err := c.dbc.Save(&artiststar).Error; err != nil {
			return spec.NewError(0, "save artist star: %v", err)
		}
	}

	for _, id := range starIDsOfType(params, specid.Track) {
		var trackstar db.TrackStar
		if err := c.dbc.Where("user_id=? AND track_id=?", user.ID, id).First(&trackstar).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(0, "find track star: %v", err)
		}
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
	default:
		return spec.NewError(0, "non-album non-artist non-track id cannot be rated")
	}

	return spec.NewResponse()
}
