//nolint:goconst
package ctrlsubsonic

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
)

// the subsonic spec mentions "artist" a lot when talking about the
// browse by folder endpoints. but since we're not browsing by tag
// we can't access artists. so instead we'll consider the artist of
// an track to be the it's respective folder that comes directly
// under the root directory

func (c *Controller) ServeGetIndexes(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	rootQ := c.dbc.
		Select("id").
		Model(&db.Album{}).
		Where("parent_id IS NULL").
		Scopes(spec.WithAlbumRootDir(getMusicFolder(c.musicPaths, params)))
	var folders []*spec.AlbumRow
	c.dbc.
		Scopes(spec.AlbumWithChildAlbumCounts, spec.AlbumWithUserData(user.ID)).
		Where("albums.parent_id IN ?", rootQ.SubQuery()).
		Order("albums.right_path COLLATE NOCASE").
		Find(&folders)
	// [a-z#] -> 27
	indexMap := make(map[string]*spec.Index, 27)
	resp := make([]*spec.Index, 0, 27)
	for _, folder := range folders {
		key := lowerUDecOrHash(folder.IndexRightPath())
		if _, ok := indexMap[key]; !ok {
			indexMap[key] = &spec.Index{
				Name:    key,
				Artists: []*spec.Artist{},
			}
			resp = append(resp, indexMap[key])
		}
		indexMap[key].Artists = append(indexMap[key].Artists, spec.NewArtistByFolder(folder))
	}
	sub := spec.NewResponse()
	sub.Indexes = &spec.Indexes{
		LastModified: 0,
		Index:        resp,
	}
	return sub
}

func (c *Controller) ServeGetMusicDirectory(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	id, err := params.GetID("id")
	if err != nil {
		return spec.NewError(10, "please provide an `id` parameter")
	}
	user := r.Context().Value(CtxUser).(*db.User)
	childrenObj := []*spec.TrackChild{}
	folder := &db.Album{}
	c.dbc.
		Scopes(spec.AlbumWithUserData(user.ID)).
		First(folder, id.Value)
	// start looking for child childFolders in the current dir
	var childFolders []*db.Album
	c.dbc.
		Scopes(spec.AlbumWithUserData(user.ID)).
		Where("parent_id=?", id.Value).
		Order("tag_year").
		Order("albums.right_path COLLATE NOCASE").
		Find(&childFolders)
	for _, ch := range childFolders {
		childrenObj = append(childrenObj, spec.NewTCAlbumByFolder(ch))
	}

	// start looking for child childTracks in the current dir
	var childTracks []*db.Track
	c.dbc.
		Scopes(spec.LoadTrackByFolder(user.ID)).
		Where("album_id=?", id.Value).
		Order("tracks.tag_disc_number, tracks.tag_track_number").
		Order("filename").
		Find(&childTracks)

	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, params.GetOr("c", ""))

	for _, ch := range childTracks {
		toAppend := spec.NewTCTrackByFolder(ch, folder)
		if v, _ := params.Get("c"); v == "Jamstash" {
			// jamstash thinks it can't play flacs
			toAppend.ContentType = "audio/mpeg"
			toAppend.Suffix = "mp3"
		}
		toAppend.TranscodeMeta = transcodeMeta
		childrenObj = append(childrenObj, toAppend)
	}
	// respond section
	sub := spec.NewResponse()
	sub.Directory = spec.NewDirectoryByFolder(folder, childrenObj)
	return sub
}

// ServeGetAlbumList handles the getAlbumList view.
// changes to this function should be reflected in _by_tags.go's
// getAlbumListTwo() function
func (c *Controller) ServeGetAlbumList(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	q := c.dbc.DB
	switch v, _ := params.Get("type"); v {
	case "alphabeticalByArtist":
		q = q.Joins(`
			JOIN albums parent_albums
			ON albums.parent_id=parent_albums.id`)
		q = q.Order("parent_albums.right_path")
	case "alphabeticalByName":
		q = q.Order("right_path")
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
		q = q.Order("right_path")
	case "frequent":
		q = q.Having("play_length > 0").Order("play_length DESC")
	case "newest":
		q = q.Order("created_at DESC")
	case "random":
		q = q.Order(gorm.Expr("random()"))
	case "recent":
		q = q.Having("play_time IS NOT NULL").Order("play_time DESC")
	case "starred":
		q = q.Joins("JOIN album_stars ON albums.id=album_stars.album_id AND album_stars.user_id=?", user.ID)
		q = q.Order("right_path")
	case "highest":
		q = q.Joins("JOIN album_ratings ON album_ratings.album_id=albums.id AND album_ratings.user_id=?", user.ID)
		q = q.Order("album_ratings.rating DESC")
	default:
		return spec.NewError(10, "unknown value %q for parameter 'type'", v)
	}

	q = q.Scopes(spec.WithAlbumRootDir(getMusicFolder(c.musicPaths, params)))
	var folders []*spec.AlbumRow
	// TODO: think about removing this extra join to count number
	// of children. it might make sense to store that in the db
	q.
		Scopes(spec.AlbumWithUserPlay(user.ID), spec.AlbumWithUserData(user.ID)).
		Joins("JOIN album_credits ON album_credits.album_id=albums.id AND album_credits.role=?", db.RoleAlbumArtist).
		Offset(params.GetOrInt("offset", 0)).
		Limit(params.GetOrInt("size", 10)).
		Preload("Parent").
		Find(&folders)
	sub := spec.NewResponse()
	sub.Albums = &spec.Albums{
		List: make([]*spec.Album, len(folders)),
	}
	for i, folder := range folders {
		sub.Albums.List[i] = spec.NewAlbumByFolder(folder)
	}
	return sub
}

func (c *Controller) ServeSearchTwo(r *http.Request) *spec.Response {
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

	musicFolder := getMusicFolder(c.musicPaths, params)
	results := &spec.SearchResultTwo{}

	// search "artists"
	rootQ := c.dbc.
		Select("id").
		Model(&db.Album{}).
		Where("parent_id IS NULL").
		Scopes(spec.WithAlbumRootDir(musicFolder))

	var artists []*db.Album
	q := c.dbc.
		Scopes(spec.AlbumWithUserData(user.ID)).
		Where(`parent_id IN ?`, rootQ.SubQuery())
	switch {
	case isUUID:
		q = q.Where(0)
	case isAll:
	default:
		q = q.Where(`right_path LIKE ? OR right_path_u_dec LIKE ?`, fuzzy, fuzzy)
	}
	q = q.
		Offset(params.GetOrInt("artistOffset", 0)).
		Limit(params.GetOrInt("artistCount", 20))
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(0, "find artists: %v", err)
	}
	for _, a := range artists {
		results.Artists = append(results.Artists, spec.NewDirectoryByFolder(a, nil))
	}

	// search "albums"
	var albums []*db.Album
	q = c.dbc.
		Scopes(spec.AlbumWithUserData(user.ID), spec.WithAlbumRootDir(musicFolder)).
		Joins("JOIN album_credits ON album_credits.album_id=albums.id AND album_credits.role=?", db.RoleAlbumArtist)
	switch {
	case isUUID:
		q = q.Where(`tag_brainz_id = ?`, query)
	case isAll:
	default:
		q = q.Where(`right_path LIKE ? OR right_path_u_dec LIKE ?`, fuzzy, fuzzy)
	}
	q = q.
		Offset(params.GetOrInt("albumOffset", 0)).
		Limit(params.GetOrInt("albumCount", 20))
	if err := q.Find(&albums).Error; err != nil {
		return spec.NewError(0, "find albums: %v", err)
	}
	for _, a := range albums {
		results.Albums = append(results.Albums, spec.NewTCAlbumByFolder(a))
	}

	// search tracks
	var tracks []*db.Track
	q = c.dbc.
		Scopes(spec.LoadTrackByFolder(user.ID))
	switch {
	case isUUID:
		q = q.Where(`tag_brainz_id = ?`, query)
	case isAll:
	default:
		q = q.Where(`filename LIKE ?`, fuzzy)
	}
	q = q.
		Offset(params.GetOrInt("songOffset", 0)).
		Limit(params.GetOrInt("songCount", 20))
	if musicFolder != "" {
		q = q.
			Joins("JOIN albums ON albums.id=tracks.album_id").
			Scopes(spec.WithAlbumRootDir(musicFolder))
	}
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find tracks: %v", err)
	}

	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, params.GetOr("c", ""))

	for _, t := range tracks {
		track := spec.NewTCTrackByFolder(t, t.Album)
		track.TranscodeMeta = transcodeMeta
		results.Tracks = append(results.Tracks, track)
	}

	sub := spec.NewResponse()
	sub.SearchResultTwo = results
	return sub
}

func (c *Controller) ServeGetArtistInfo(_ *http.Request) *spec.Response {
	return spec.NewResponse()
}

func (c *Controller) ServeGetStarred(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	musicFolder := getMusicFolder(c.musicPaths, params)

	results := &spec.Starred{}

	// "artists"
	rootQ := c.dbc.
		Select("id").
		Model(&db.Album{}).
		Where("parent_id IS NULL").
		Scopes(spec.WithAlbumRootDir(musicFolder))

	var artists []*db.Album
	q := c.dbc.
		Scopes(spec.AlbumWithUserData(user.ID)).
		Where(`parent_id IN ?`, rootQ.SubQuery()).
		Joins("JOIN album_stars ON albums.id=album_stars.album_id").
		Where("album_stars.user_id=?", user.ID)
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(0, "find artists: %v", err)
	}
	for _, a := range artists {
		results.Artists = append(results.Artists, spec.NewDirectoryByFolder(a, nil))
	}

	// "albums"
	var albums []*db.Album
	q = c.dbc.
		Scopes(spec.AlbumWithUserData(user.ID), spec.WithAlbumRootDir(musicFolder)).
		Joins("JOIN album_credits ON album_credits.album_id=albums.id AND album_credits.role=?", db.RoleAlbumArtist).
		Joins("JOIN album_stars ON albums.id=album_stars.album_id").
		Where("album_stars.user_id=?", user.ID)
	if err := q.Find(&albums).Error; err != nil {
		return spec.NewError(0, "find albums: %v", err)
	}
	for _, a := range albums {
		results.Albums = append(results.Albums, spec.NewTCAlbumByFolder(a))
	}

	// tracks
	var tracks []*db.Track
	q = c.dbc.
		Scopes(spec.LoadTrackByFolder(user.ID)).
		Joins("JOIN track_stars ON tracks.id=track_stars.track_id").
		Where("track_stars.user_id=?", user.ID)
	if musicFolder != "" {
		q = q.
			Joins("JOIN albums ON albums.id=tracks.album_id").
			Scopes(spec.WithAlbumRootDir(musicFolder))
	}
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find tracks: %v", err)
	}

	transcodeMeta := streamGetTranscodeMeta(c.dbc, user.ID, params.GetOr("c", ""))

	for _, t := range tracks {
		track := spec.NewTCTrackByFolder(t, t.Album)
		track.TranscodeMeta = transcodeMeta
		results.Tracks = append(results.Tracks, track)
	}

	sub := spec.NewResponse()
	sub.Starred = results
	return sub
}
