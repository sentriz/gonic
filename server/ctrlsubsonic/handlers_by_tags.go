package ctrlsubsonic

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scrobble/lastfm"
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
		a := spec.NewArtistByTags(artist)
		c.addStarRatingToArtist(user.ID, a)
		indexMap[key].Artists = append(indexMap[key].Artists, a)
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
				Order("albums.right_path").
				Group("albums.id")
		}).
		First(artist, id.Value)
	sub := spec.NewResponse()
	sub.Artist = spec.NewArtistByTags(artist)
	c.addStarRatingToArtist(user.ID, sub.Artist)
	sub.Artist.Albums = make([]*spec.Album, len(artist.Albums))
	for i, album := range artist.Albums {
		sub.Artist.Albums[i] = spec.NewAlbumByTags(album, artist)
		c.addStarRatingToAlbum(user.ID, sub.Artist.Albums[i])
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
			return db.Order("tracks.tag_disc_number, tracks.tag_track_number")
		}).
		First(album, id.Value).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(10, "couldn't find an album with that id")
	}
	sub := spec.NewResponse()
	sub.Album = spec.NewAlbumByTags(album, album.TagArtist)
	c.addStarRatingToAlbum(user.ID, sub.Album)
	sub.Album.Tracks = make([]*spec.TrackChild, len(album.Tracks))
	for i, track := range album.Tracks {
		t := spec.NewTrackByTags(track, album)
		c.addStarRatingToTCTrack(user.ID, t)
		sub.Album.Tracks[i] = t
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
		a := spec.NewAlbumByTags(album, album.TagArtist)
		c.addStarRatingToAlbum(user.ID, a)
		sub.AlbumsTwo.List[i] = a
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
	query = fmt.Sprintf("%%%s%%", strings.Trim(query, `*"'`))
	results := &spec.SearchResultThree{}

	// search "artists"
	var artists []*db.Artist
	q := c.DB.
		Select("*, count(albums.id) album_count").
		Group("artists.id").
		Where("name LIKE ? OR name_u_dec LIKE ?", query, query).
		Joins("JOIN albums ON albums.tag_artist_id=artists.id").
		Offset(params.GetOrInt("artistOffset", 0)).
		Limit(params.GetOrInt("artistCount", 20))
	if m := c.getMusicFolder(params); m != "" {
		q = q.Where("albums.root_dir=?", m)
	}
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(0, "find artists: %v", err)
	}
	for _, a := range artists {
		ar := spec.NewArtistByTags(a)
		c.addStarRatingToArtist(user.ID, ar)
		results.Artists = append(results.Artists, ar)
	}

	// search "albums"
	var albums []*db.Album
	q = c.DB.
		Preload("TagArtist").
		Preload("Genres").
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
		al := spec.NewAlbumByTags(a, a.TagArtist)
		c.addStarRatingToAlbum(user.ID, al)
		results.Albums = append(results.Albums, al)
	}

	// search tracks
	var tracks []*db.Track
	q = c.DB.
		Preload("Album").
		Preload("Album.TagArtist").
		Preload("Genres").
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
		tr := spec.NewTrackByTags(t, t.Album)
		c.addStarRatingToTCTrack(user.ID, tr)
		results.Tracks = append(results.Tracks, tr)
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
	info, err := lastfm.ArtistGetInfo(apiKey, artist.Name)
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
	}

	count := params.GetOrInt("count", 20)
	inclNotPresent := params.GetOrBool("includeNotPresent", false)
	similarArtists, err := lastfm.ArtistGetSimilar(apiKey, artist.Name)
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
		tr := spec.NewTrackByTags(track, track.Album)
		c.addStarRatingToTCTrack(user.ID, tr)
		sub.TracksByGenre.List[i] = tr
	}
	return sub
}

func (c *Controller) ServeGetStarredTwo(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)

	sub := spec.NewResponse()
	sub.StarredTwo = &spec.StarredTwo{
		Artists: []*spec.Artist{},
		Albums:  []*spec.Album{},
		Tracks:  []*spec.TrackChild{},
	}

	// artists
	var artists []*db.Artist
	q := c.DB.Table("artists").Joins("right join artiststars on artists.id=artiststars.artistid").Where("artiststars.userid=?", user.ID)
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(0, "find artists: %v", err)
	}
	for _, a := range artists {
		artist := spec.NewArtistByTags(a)
		c.addStarRatingToArtist(user.ID, artist)
		sub.StarredTwo.Artists = append(sub.StarredTwo.Artists, artist)
	}

	// albums
	var albums []*db.Album
	q = c.DB.Table("albums").Joins("right join albumstars on albums.id=albumstars.albumid").Where("albumstars.userid=?", user.ID)
	if err := q.Find(&albums).Error; err != nil {
		return spec.NewError(0, "find albums: %v", err)
	}
	for _, a := range albums {
		var ar db.Artist
		if err := c.DB.Where("artistid=?",a.TagArtistID).Find(&ar); err != nil {
			return spec.NewError(0, "find artist for album: %v", err)
		}
		album := spec.NewAlbumByTags(a, &ar)
		c.addStarRatingToAlbum(user.ID, album)
		sub.StarredTwo.Albums = append(sub.StarredTwo.Albums, album)
	}

	// tracks
	var tracks []*db.Track
	q = c.DB.Table("tracks").Joins("right join trackstars on tracks.id=trackstars.trackid").Where("trackstars.userid=?", user.ID)
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find tracks: %v", err)
	}
	for _, t := range tracks {
		var a db.Album
		if err := c.DB.Where("albumid=?",t.AlbumID).Find(&a); err != nil {
			return spec.NewError(0, "find album for track: %v", err)
		}
		track := spec.NewTCTrackByFolder(t, &a)
		c.addStarRatingToTCTrack(user.ID, track)
		sub.StarredTwo.Tracks = append(sub.StarredTwo.Tracks, track)
	}

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
	topTracks, err := lastfm.ArtistGetTopTracks(apiKey, artist.Name)
	if err != nil {
		return spec.NewError(0, "fetching artist top tracks: %v", err)
	}
	if len(topTracks.Tracks) == 0 {
		return spec.NewError(70, "no top tracks found for artist: %v", artist)
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
		Find(&tracks).
		Error
	if err != nil {
		return spec.NewError(0, "error finding tracks: %v", err)
	}
	if len(tracks) == 0 {
		return spec.NewError(70, "no tracks found matchind last fm top songs for artist: %v", artist)
	}

	sub := spec.NewResponse()
	sub.TopSongs = &spec.TopSongs{
		Tracks: make([]*spec.TrackChild, len(tracks)),
	}
	for i, track := range tracks {
		tr := spec.NewTrackByTags(track, track.Album)
		c.addStarRatingToTCTrack(user.ID, tr)
		sub.TopSongs.Tracks[i] = tr
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

	similarTracks, err := lastfm.TrackGetSimilarTracks(apiKey, track.Artist.Name, track.TagTitle)
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
	for i, track := range tracks {
		tr := spec.NewTrackByTags(track, track.Album)
		c.addStarRatingToTCTrack(user.ID, tr)
		sub.SimilarSongs.Tracks[i] = tr
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

	similarArtists, err := lastfm.ArtistGetSimilar(apiKey, artist.Name)
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
	for i, track := range tracks {
		tr := spec.NewTrackByTags(track, track.Album)
		c.addStarRatingToTCTrack(user.ID, tr)
		sub.SimilarSongsTwo.Tracks[i] = tr
	}
	return sub
}

func (c* Controller) parseStarParams(p params.Params) (*spec.Response, []int, []int, []int) {
	var albumIDs []int
	var artistIDs []int
	var trackIDs []int

	if idl, err := p.GetIDList("id"); err == nil {
		for _, i := range idl {
			switch i.Type {
			case specid.Album:
				albumIDs = append(albumIDs, i.Value)
			case specid.Artist:
				artistIDs = append(artistIDs, i.Value)
			case specid.Track:
				trackIDs = append(trackIDs, i.Value)
			default:
				return spec.NewError(0, "non-album non-artist non-track id cannot be starred"), nil, nil, nil
			}
		}
	}

	if idl, err := p.GetIDList("albumId"); err == nil {
		for _, i := range idl {
			switch i.Type {
			case specid.Album:
				albumIDs = append(albumIDs, i.Value)
			default:
				return spec.NewError(0, "non-album id passed as albumId"), nil, nil, nil
			}
		}
	}

	if idl, err := p.GetIDList("artistId"); err == nil {
		for _, i := range idl {
			switch i.Type {
			case specid.Artist:
				artistIDs = append(artistIDs, i.Value)
			default:
				return spec.NewError(0, "non-artist id passed as artistId"), nil, nil, nil
			}
		}
	}

	return nil, albumIDs, artistIDs, trackIDs
}

func (c *Controller) ServeStar(r *http.Request) *spec.Response {
	p := r.Context().Value(CtxParams).(params.Params)
	response, albumIDs, artistIDs, trackIDs := c.parseStarParams(p)
	if response != nil {
		return response;
	}
	
	stardate := time.Now()
	user := r.Context().Value(CtxUser).(*db.User)
	for _, i := range albumIDs {
		var albumstar db.AlbumStar
		_ = c.DB.Where("userid=? AND albumid=?", user.ID, i).First(&albumstar).Error
		albumstar.UserID = user.ID
		albumstar.AlbumID = i
		albumstar.StarDate = stardate
		if err := c.DB.Save(&albumstar).Error; err != nil {
			return spec.NewError(0, "save album star: %v", err)
		}
	}

	for _, i := range artistIDs {
		var artiststar db.ArtistStar
		_ = c.DB.Where("userid=? AND artistid=?", user.ID, i).First(&artiststar).Error
		artiststar.UserID = user.ID
		artiststar.ArtistID = i
		artiststar.StarDate = stardate
		if err := c.DB.Save(&artiststar).Error; err != nil {
			return spec.NewError(0, "save artist star: %v", err)
		}
	}

	for _, i := range trackIDs {
		var trackstar db.TrackStar
		_ = c.DB.Where("userid=? AND trackid=?", user.ID, i).First(&trackstar).Error
		trackstar.UserID = user.ID
		trackstar.TrackID = i
		trackstar.StarDate = stardate
		if err := c.DB.Save(&trackstar).Error; err != nil {
			return spec.NewError(0, "save track star: %v", err)
		}
	}

	return spec.NewResponse()
}

func (c *Controller) ServeUnstar(r *http.Request) *spec.Response {
	p := r.Context().Value(CtxParams).(params.Params)
	response, albumIDs, artistIDs, trackIDs := c.parseStarParams(p)
	if response != nil {
		return response;
	}
	
	user := r.Context().Value(CtxUser).(*db.User)
	for _, i := range albumIDs {
		var albumstar db.AlbumStar
		err := c.DB.Where("userid=? AND albumid=?", user.ID, i).First(&albumstar).Error
		if (err != nil) {
			if err := c.DB.Delete(&albumstar).Error; err != nil {
				return spec.NewError(0, "delete album star: %v", err)
			}
		}
	}

	for _, i := range artistIDs {
		var artiststar db.ArtistStar
		err := c.DB.Where("userid=? AND artistid=?", user.ID, i).First(&artiststar).Error
		if (err != nil) {
			if err := c.DB.Delete(&artiststar).Error; err != nil {
				return spec.NewError(0, "delete artist star: %v", err)
			}
		}
	}

	for _, i := range trackIDs {
		var trackstar db.TrackStar
		err := c.DB.Where("userid=? AND trackid=?", user.ID, i).First(&trackstar).Error
		if (err != nil) {
			if err := c.DB.Delete(&trackstar).Error; err != nil {
				return spec.NewError(0, "delete track star: %v", err)
			}
		}
	}

	return spec.NewResponse()
}

func (c *Controller) ServeSetRating(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	var err error
	var id specid.ID
	var rating int

	if id, err = params.GetID("id"); err != nil {
		return spec.NewError(10, "please provide a valid id")
	}

	if rating, err = params.GetInt("rating"); ((err != nil) || (rating < 0) || (rating > 5)) {
		return spec.NewError(10, "please provide a valid rating")
	}

	user := r.Context().Value(CtxUser).(*db.User)

	switch id.Type {
	case specid.Album:
		var albumrating db.AlbumRating
		err := c.DB.Where("userid=? AND albumid=?", user.ID, id.Value).First(&albumrating).Error
		if (rating != 0) {
			albumrating.UserID = user.ID
			albumrating.AlbumID = id.Value
			albumrating.Rating = rating
			if err := c.DB.Save(&albumrating).Error; err != nil {
				return spec.NewError(0, "save album rating: %v", err)
			}
		} else {
			if (err == nil) {
				if err := c.DB.Delete(&albumrating).Error; err != nil {
					return spec.NewError(0, "delete album rating: %v", err)
				}
			}
		}
	case specid.Artist:
		var artistrating db.ArtistRating
		err := c.DB.Where("userid=? AND artistid=?", user.ID, id.Value).First(&artistrating).Error
		if (rating != 0) {
			artistrating.UserID = user.ID
			artistrating.ArtistID = id.Value
			artistrating.Rating = rating
			if err := c.DB.Save(&artistrating).Error; err != nil {
				return spec.NewError(0, "save artist rating: %v", err)
			}
		} else {
			if (err == nil) {
				if err := c.DB.Delete(&artistrating).Error; err != nil {
					return spec.NewError(0, "delete artist rating: %v", err)
				}
			}
		}
	case specid.Track:
		var trackrating db.TrackRating
		err := c.DB.Where("userid=? AND trackid=?", user.ID, id.Value).First(&trackrating).Error
		if (rating != 0) {
			trackrating.UserID = user.ID
			trackrating.TrackID = id.Value
			trackrating.Rating = rating
			if err := c.DB.Save(&trackrating).Error; err != nil {
				return spec.NewError(0, "save track rating: %v", err)
			}
		} else {
			if (err == nil) {
				if err := c.DB.Delete(&trackrating).Error; err != nil {
					return spec.NewError(0, "delete track rating: %v", err)
				}
			}
		}
	default:
		return spec.NewError(0, "non-album non-artist non-track id cannot be rated")
	}

	return spec.NewResponse()
}
