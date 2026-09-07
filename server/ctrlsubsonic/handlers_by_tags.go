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
	rnd := render(c, r)
	q := c.dbc.
		Select("artists.*").
		Group("artists.id").
		Joins("JOIN album_credits ON album_credits.artist_id=artists.id AND album_credits.role=?", db.RoleAlbumArtist).
		Order("artists.name COLLATE NOCASE")
	if rnd.MusicFolder != "" {
		q = q.
			Joins("JOIN albums ON albums.id=album_credits.album_id").
			Scopes(spec.WithAlbumRootDir(rnd.MusicFolder))
	}
	var artists []*db.Artist
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(10, "error finding artists: %v", err)
	}
	rendered, err := spec.ArtistsByTags(rnd, artists)
	if err != nil {
		return spec.NewError(0, "render artists: %v", err)
	}
	// [a-z#] -> 27
	indexMap := make(map[string]*spec.Index, 27)
	resp := make([]*spec.Index, 0, 27)
	for i, artist := range artists {
		key := lowerUDecOrHash(artist.IndexName())
		if _, ok := indexMap[key]; !ok {
			indexMap[key] = &spec.Index{
				Name:    key,
				Artists: []*spec.Artist{},
			}
			resp = append(resp, indexMap[key])
		}
		indexMap[key].Artists = append(indexMap[key].Artists, rendered[i])
	}
	sub := spec.NewResponse()
	sub.Artists = &spec.Artists{
		List: resp,
	}
	return sub
}

func (c *Controller) ServeGetArtist(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	rnd := render(c, r)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	artist := &db.Artist{}
	if err := c.dbc.Where("artists.id=?", id.Value).Limit(1).Find(artist).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return spec.NewError(70, "couldn't find an artist with that id")
		}
		return spec.NewError(0, "find artist: %v", err)
	}

	var appearances []*db.Album
	if err := c.dbc.
		Select("albums.*").
		Where(`albums.id IN (
			SELECT album_id FROM album_credits WHERE artist_id=?
			UNION
			SELECT tracks.album_id FROM track_credits
				JOIN tracks ON tracks.id=track_credits.track_id
				WHERE track_credits.artist_id=?
		)`, artist.ID, artist.ID).
		Order("albums.right_path").
		Find(&appearances).Error; err != nil {
		return spec.NewError(0, "find artist appearances: %v", err)
	}

	rendered, err := spec.ArtistsByTags(rnd, []*db.Artist{artist})
	if err != nil {
		return spec.NewError(0, "render artist: %v", err)
	}
	albums, err := spec.AlbumsByTags(rnd, appearances)
	if err != nil {
		return spec.NewError(0, "render appearances: %v", err)
	}
	sub := spec.NewResponse()
	sub.Artist = rendered[0]
	sub.Artist.Albums = albums
	sub.Artist.AlbumCount = len(appearances)
	return sub
}

func (c *Controller) ServeGetAlbum(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	rnd := render(c, r)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	album := &db.Album{}
	err = c.dbc.
		Select("albums.*").
		Where("albums.id=?", id.Value).
		Limit(1).
		Find(album).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(70, "couldn't find an album with that id")
	}
	if err != nil {
		return spec.NewError(0, "find album: %v", err)
	}

	var tracks []*db.Track
	if err := c.dbc.
		Select("tracks.*").
		Where("album_id=?", id.Value).
		Order("tracks.tag_disc_number, tracks.tag_track_number").
		Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find album tracks: %v", err)
	}
	children, err := spec.TrackChildrenByTags(rnd, tracks)
	if err != nil {
		return spec.NewError(0, "render tracks: %v", err)
	}
	albums, err := spec.AlbumsByTags(rnd, []*db.Album{album})
	if err != nil {
		return spec.NewError(0, "render album: %v", err)
	}

	sub := spec.NewResponse()
	sub.Album = albums[0]
	sub.Album.Tracks = children
	return sub
}

// ServeGetAlbumListTwo handles the getAlbumList2 view.
// changes to this function should be reflected in _by_folder.go's
// getAlbumList() function
func (c *Controller) ServeGetAlbumListTwo(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	rnd := render(c, r)
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
		q = q.Scopes(spec.AlbumWithUserPlay(user.ID)).Having("play_length > 0").Order("play_length DESC")
	case "newest":
		q = q.Order("albums.created_at DESC")
	case "random":
		q = q.Order(gorm.Expr("random()"))
	case "recent":
		q = q.Scopes(spec.AlbumWithUserPlay(user.ID)).Having("play_time IS NOT NULL").Order("play_time DESC")
	case "starred":
		q = q.Joins("JOIN album_stars ON albums.id=album_stars.album_id AND album_stars.user_id=?", user.ID)
		q = q.Order("albums.tag_title")
	case "highest":
		q = q.Joins("JOIN album_ratings ON album_ratings.album_id=albums.id AND album_ratings.user_id=?", user.ID)
		q = q.Order("album_ratings.rating DESC")
	default:
		return spec.NewError(10, "unknown value %q for parameter 'type'", listType)
	}
	q = q.Scopes(spec.WithAlbumRootDir(rnd.MusicFolder))
	var albums []*db.Album
	if err := q.
		Select("albums.*").
		Joins("JOIN album_credits ON album_credits.album_id=albums.id AND album_credits.role=?", db.RoleAlbumArtist).
		Group("albums.id").
		Order("albums.id"). // tiebreak so equal sort values keep a stable order across pages
		Offset(params.GetOrInt("offset", 0)).
		Limit(params.GetOrInt("size", 10)).
		Find(&albums).Error; err != nil {
		return spec.NewError(0, "error finding albums: %v", err)
	}
	rendered, err := spec.AlbumsByTags(rnd, albums)
	if err != nil {
		return spec.NewError(0, "render albums: %v", err)
	}
	sub := spec.NewResponse()
	sub.AlbumsTwo = &spec.Albums{List: rendered}
	return sub
}

func (c *Controller) ServeSearchThree(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	rnd := render(c, r)
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

	// search artists
	q := c.dbc.
		Select("artists.*").
		Group("artists.id")
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
		Scopes(spec.WithAlbumRootDir(rnd.MusicFolder)).
		Offset(params.GetOrInt("artistOffset", 0)).
		Limit(params.GetOrInt("artistCount", 20))
	var artists []*db.Artist
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(0, "find artists: %v", err)
	}
	renderedArtists, err := spec.ArtistsByTags(rnd, artists)
	if err != nil {
		return spec.NewError(0, "render artists: %v", err)
	}
	results.Artists = append(results.Artists, renderedArtists...)

	// search albums
	q = c.dbc.
		Select("albums.*").
		Scopes(spec.WithAlbumRootDir(rnd.MusicFolder))
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
	var albums []*db.Album
	if err := q.Find(&albums).Error; err != nil {
		return spec.NewError(0, "find albums: %v", err)
	}
	renderedAlbums, err := spec.AlbumsByTags(rnd, albums)
	if err != nil {
		return spec.NewError(0, "render albums: %v", err)
	}
	results.Albums = append(results.Albums, renderedAlbums...)

	// search tracks
	q = c.dbc.
		Select("tracks.*")
	switch {
	case isUUID:
		q = q.Where(`tracks.tag_brainz_id = ?`, query)
	case isAll:
	default:
		q = q.Where(`tracks.tag_title LIKE ? OR tracks.tag_title_u_dec LIKE ?`, fuzzy, fuzzy)
	}
	q = q.Offset(params.GetOrInt("songOffset", 0)).
		Limit(params.GetOrInt("songCount", 20))
	if rnd.MusicFolder != "" {
		q = q.
			Joins("JOIN albums ON albums.id=tracks.album_id").
			Scopes(spec.WithAlbumRootDir(rnd.MusicFolder))
	}
	var tracks []*db.Track
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find tracks: %v", err)
	}

	renderedTracks, err := spec.TrackChildrenByTags(rnd, tracks)
	if err != nil {
		return spec.NewError(0, "render tracks: %v", err)
	}
	results.Tracks = append(results.Tracks, renderedTracks...)

	sub := spec.NewResponse()
	sub.SearchResultThree = results
	return sub
}

func (c *Controller) ServeGetArtistInfoTwo(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	rnd := render(c, r)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}

	var artist db.Artist
	err = c.dbc.
		Where("artists.id=?", id.Value).
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

	coverID := artist.SID()
	if info.ImageURL == "" {
		rendered, err := spec.ArtistsByTags(rnd, []*db.Artist{&artist})
		if err != nil {
			return spec.NewError(0, "render artist: %v", err)
		}
		if rendered[0].CoverID != nil {
			coverID = rendered[0].CoverID
		}
	}

	sub.ArtistInfoTwo.SmallImageURL = c.genArtistCoverURL(r, coverID, 64)
	sub.ArtistInfoTwo.MediumImageURL = c.genArtistCoverURL(r, coverID, 126)
	sub.ArtistInfoTwo.LargeImageURL = c.genArtistCoverURL(r, coverID, 256)

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

	var localArtists []*db.Artist
	var localPositions []int
	for i, similarName := range similar {
		if i == count {
			break
		}
		artist := &db.Artist{}
		err = c.dbc.
			Where("name=?", similarName).
			Limit(1).
			Find(artist).
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

		localArtists = append(localArtists, artist)
		localPositions = append(localPositions, len(sub.ArtistInfoTwo.Similar))
		sub.ArtistInfoTwo.Similar = append(sub.ArtistInfoTwo.Similar, nil)
	}

	rendered, err := spec.ArtistsByTags(rnd, localArtists)
	if err != nil {
		return spec.NewError(0, "render similar artists: %v", err)
	}
	for i, artist := range rendered {
		sub.ArtistInfoTwo.Similar[localPositions[i]] = artist
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
	var genres []*db.Genre
	if err := c.dbc.
		Order("genres.name").
		Find(&genres).Error; err != nil {
		return spec.NewError(0, "error finding genres: %v", err)
	}
	rendered, err := spec.GenresWithCounts(c.dbc, genres)
	if err != nil {
		return spec.NewError(0, "render genres: %v", err)
	}
	sub := spec.NewResponse()
	sub.Genres = &spec.Genres{List: rendered}
	return sub
}

func (c *Controller) ServeGetSongsByGenre(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	rnd := render(c, r)
	genre, err := params.Get("genre")
	if err != nil {
		return spec.NewError(10, "please provide an `genre` parameter")
	}
	q := c.dbc.
		Select("tracks.*").
		Scopes(spec.WithAlbumRootDir(rnd.MusicFolder)).
		Joins("JOIN albums ON tracks.album_id=albums.id").
		Joins("JOIN track_genres ON track_genres.track_id=tracks.id").
		Joins("JOIN genres ON track_genres.genre_id=genres.id AND genres.name=?", genre).
		Offset(params.GetOrInt("offset", 0)).
		Limit(params.GetOrInt("count", 10)).
		Group("tracks.id")
	var tracks []*db.Track
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "error finding tracks: %v", err)
	}
	sub := spec.NewResponse()
	rendered, err := spec.TrackChildrenByTags(rnd, tracks)
	if err != nil {
		return spec.NewError(0, "render tracks: %v", err)
	}
	sub.TracksByGenre = &spec.TracksByGenre{List: rendered}

	return sub
}

func (c *Controller) ServeGetStarredTwo(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	rnd := render(c, r)

	results := &spec.StarredTwo{}

	// artists
	q := c.dbc.
		Select("artists.*").
		Group("artists.id").
		Scopes(spec.WithAlbumRootDir(rnd.MusicFolder)).
		Joins("JOIN artist_stars ON artist_stars.artist_id=artists.id").
		Where("artist_stars.user_id=?", user.ID).
		Joins("JOIN album_credits ON album_credits.artist_id=artists.id AND album_credits.role=?", db.RoleAlbumArtist).
		Joins("JOIN albums ON albums.id=album_credits.album_id").
		Order("artist_stars.star_date DESC")
	var artists []*db.Artist
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(0, "find artists: %v", err)
	}
	renderedArtists, err := spec.ArtistsByTags(rnd, artists)
	if err != nil {
		return spec.NewError(0, "render artists: %v", err)
	}
	results.Artists = append(results.Artists, renderedArtists...)

	// albums
	q = c.dbc.
		Select("albums.*").
		Scopes(spec.WithAlbumRootDir(rnd.MusicFolder)).
		Joins("JOIN album_stars ON album_stars.album_id=albums.id").
		Where("album_stars.user_id=?", user.ID).
		Order("album_stars.star_date DESC")
	var albums []*db.Album
	if err := q.Find(&albums).Error; err != nil {
		return spec.NewError(0, "find albums: %v", err)
	}
	renderedAlbums, err := spec.AlbumsByTags(rnd, albums)
	if err != nil {
		return spec.NewError(0, "render albums: %v", err)
	}
	results.Albums = append(results.Albums, renderedAlbums...)

	// tracks
	q = c.dbc.
		Select("tracks.*").
		Joins("JOIN track_stars ON tracks.id=track_stars.track_id").
		Where("track_stars.user_id=?", user.ID).
		Order("track_stars.star_date DESC")
	if rnd.MusicFolder != "" {
		q = q.
			Joins("JOIN albums ON albums.id=tracks.album_id").
			Scopes(spec.WithAlbumRootDir(rnd.MusicFolder))
	}
	var tracks []*db.Track
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find tracks: %v", err)
	}

	renderedTracks, err := spec.TrackChildrenByTags(rnd, tracks)
	if err != nil {
		return spec.NewError(0, "render tracks: %v", err)
	}
	results.Tracks = append(results.Tracks, renderedTracks...)

	sub := spec.NewResponse()
	sub.StarredTwo = results
	return sub
}

func (c *Controller) genArtistCoverURL(r *http.Request, id *specid.ID, size int) string {
	coverURL, _ := url.Parse(handlerutil.BaseURL(r))
	coverURL.Path = c.resolveProxyPath("/rest/getCoverArt")

	query := r.URL.Query()
	query.Set("id", id.String())
	query.Set("size", strconv.Itoa(size))
	coverURL.RawQuery = query.Encode()

	return coverURL.String()
}

func (c *Controller) ServeGetTopSongs(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	rnd := render(c, r)
	count := params.GetOrInt("count", 10)

	var artist db.Artist
	switch id, err := params.GetID("id"); {
	case err == nil && id.Type != specid.Artist:
		return spec.NewError(10, "please provide an artist `id` parameter")
	case err == nil:
		if err := c.dbc.Where("id=?", id.Value).Find(&artist).Error; err != nil {
			return spec.NewError(0, "finding artist by id: %v", err)
		}
	default:
		artistName, err := params.Get("artist")
		if err != nil {
			return spec.NewError(10, "please provide an `artist` or `id` parameter")
		}
		if err := c.dbc.Where("name=?", artistName).Find(&artist).Error; err != nil {
			return spec.NewError(0, "finding artist by name: %v", err)
		}
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

	var tracks []*db.Track
	if err := c.dbc.
		Select("tracks.*").
		Where("tracks.tag_title IN (?)", topTrackNames).
		Joins("JOIN track_credits ON track_credits.track_id=tracks.id AND track_credits.role=?", db.RoleArtist).
		Joins("JOIN artists ON artists.id=track_credits.artist_id").
		Where("artists.id=?", artist.ID).
		Group("tracks.id").
		Limit(count).
		Find(&tracks).Error; err != nil {
		return spec.NewError(0, "error finding tracks: %v", err)
	}
	if len(tracks) == 0 {
		return sub
	}

	rendered, err := spec.TrackChildrenByTags(rnd, tracks)
	if err != nil {
		return spec.NewError(0, "render tracks: %v", err)
	}
	sub.TopSongs.Tracks = append(sub.TopSongs.Tracks, rendered...)
	return sub
}

func (c *Controller) ServeGetSimilarSongs(r *http.Request) *spec.Response {
	rnd := render(c, r)
	params := r.Context().Value(CtxParams).(params.Params)
	count := params.GetOrInt("count", 10)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}

	var tracks []*spec.TrackChild
	var sub *spec.Response

	switch id.Type {
	case specid.Track:
		tracks, sub = getSimilarSongsFromTrack(c, rnd, id, count)
	case specid.Album:
		tracks, sub = getSimilarSongsFromAlbum(c, rnd, id, count)
	case specid.Artist:
		tracks, sub = getSimilarSongsFromArtist(c, rnd, id, count)
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
	rnd := render(c, r)
	count := params.GetOrInt("count", 10)
	id, err := params.GetID("id")
	if err != nil || id.Type != specid.Artist {
		return spec.NewError(10, "please provide an artist `id` parameter")
	}

	tracks, sub := getSimilarSongsFromArtist(c, rnd, id, count)
	if sub != nil {
		return sub
	}

	sub = spec.NewResponse()
	sub.SimilarSongsTwo = &spec.SimilarSongsTwo{
		Tracks: tracks,
	}
	return sub
}

func getSimilarSongsFromTrack(c *Controller, rnd spec.Render, id specid.ID, count int) ([]*spec.TrackChild, *spec.Response) {
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

	titleArtistPairs := make([][]any, 0, len(similarTracks.Tracks))
	for _, t := range similarTracks.Tracks {
		titleArtistPairs = append(titleArtistPairs, []any{t.Name, t.Artist.Name})
	}

	var tracks []*db.Track
	if err := c.dbc.
		Select("tracks.*").
		Where("(tracks.tag_title, tracks.tag_track_artist) IN (?)", titleArtistPairs).
		Order(gorm.Expr("random()")).
		Limit(count).
		Find(&tracks).Error; err != nil {
		return nil, spec.NewError(0, "error finding tracks: %v", err)
	}
	if len(tracks) == 0 {
		return nil, spec.NewError(70, "no similar song could be match with collection in database: %v", track.TagTitle)
	}

	trackChildren, err := spec.TrackChildrenByTags(rnd, tracks)
	if err != nil {
		return nil, spec.NewError(0, "render tracks: %v", err)
	}

	return trackChildren, nil
}

func getSimilarSongsFromArtist(c *Controller, rnd spec.Render, id specid.ID, count int) ([]*spec.TrackChild, *spec.Response) {
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

	var tracks []*db.Track
	if err := c.dbc.
		Select("tracks.*").
		Joins("JOIN track_credits ON track_credits.track_id=tracks.id AND track_credits.role=?", db.RoleArtist).
		Joins("JOIN artists ON artists.id=track_credits.artist_id").
		Where("artists.name IN (?)", artistNames).
		Order(gorm.Expr("random()")).
		Group("tracks.id").
		Limit(count).
		Find(&tracks).Error; err != nil {
		return nil, spec.NewError(0, "error finding tracks: %v", err)
	}
	if len(tracks) == 0 {
		return nil, spec.NewError(70, "no similar song could be match with collection in database: %v", artist.Name)
	}

	trackChildren, err := spec.TrackChildrenByTags(rnd, tracks)
	if err != nil {
		return nil, spec.NewError(0, "render tracks: %v", err)
	}

	return trackChildren, nil
}

func getSimilarSongsFromAlbum(c *Controller, rnd spec.Render, id specid.ID, count int) ([]*spec.TrackChild, *spec.Response) {
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

	titleArtistPairs := make([][]any, 0, len(similarTracks.Tracks))
	for _, t := range similarTracks.Tracks {
		titleArtistPairs = append(titleArtistPairs, []any{t.Name, t.Artist.Name})
	}

	var tracks []*db.Track
	if err := c.dbc.
		Select("tracks.*").
		Where("(tracks.tag_title, tracks.tag_track_artist) IN (?)", titleArtistPairs).
		Order(gorm.Expr("random()")).
		Limit(count).
		Find(&tracks).Error; err != nil {
		return nil, spec.NewError(0, "error finding tracks: %v", err)
	}
	if len(tracks) == 0 {
		return nil, spec.NewError(70, "no similar songs could be matched with collection in database: %v", album.TagTitle)
	}

	trackChildren, err := spec.TrackChildrenByTags(rnd, tracks)
	if err != nil {
		return nil, spec.NewError(0, "render tracks: %v", err)
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
