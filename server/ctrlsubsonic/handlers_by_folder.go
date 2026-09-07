//nolint:goconst
package ctrlsubsonic

import (
	"errors"
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
	rnd := render(c, r)
	rootQ := c.dbc.
		Select("id").
		Model(&db.Album{}).
		Where("parent_id IS NULL").
		Scopes(spec.WithAlbumRootDir(rnd.MusicFolder))
	var folders []*db.Album
	if err := c.dbc.
		Select("albums.*").
		Where("albums.parent_id IN ?", rootQ.SubQuery()).
		Order("albums.right_path COLLATE NOCASE").
		Find(&folders).Error; err != nil {
		return spec.NewError(0, "error finding folders: %v", err)
	}
	artists, err := spec.ArtistsByFolder(rnd, folders)
	if err != nil {
		return spec.NewError(0, "render folders: %v", err)
	}
	// [a-z#] -> 27
	indexMap := make(map[string]*spec.Index, 27)
	resp := make([]*spec.Index, 0, 27)
	for i, folder := range folders {
		key := lowerUDecOrHash(folder.IndexRightPath())
		if _, ok := indexMap[key]; !ok {
			indexMap[key] = &spec.Index{
				Name:    key,
				Artists: []*spec.Artist{},
			}
			resp = append(resp, indexMap[key])
		}
		indexMap[key].Artists = append(indexMap[key].Artists, artists[i])
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
	rnd := render(c, r)
	childrenObj := []*spec.TrackChild{}
	folder := &db.Album{}
	err = c.dbc.
		Select("albums.*").
		Where("albums.id=?", id.Value).
		Limit(1).
		Find(folder).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return spec.NewError(70, "couldn't find a directory with that id")
	}
	if err != nil {
		return spec.NewError(0, "error finding directory: %v", err)
	}
	// start looking for child childFolders in the current dir
	var childFolders []*db.Album
	if err := c.dbc.
		Select("albums.*").
		Where("parent_id=?", id.Value).
		Order("tag_year").
		Order("albums.right_path COLLATE NOCASE").
		Find(&childFolders).Error; err != nil {
		return spec.NewError(0, "error finding child folders: %v", err)
	}
	childAlbums, err := spec.TCAlbumsByFolder(rnd, childFolders)
	if err != nil {
		return spec.NewError(0, "render child folders: %v", err)
	}
	childrenObj = append(childrenObj, childAlbums...)

	// start looking for child childTracks in the current dir
	var childTracks []*db.Track
	if err := c.dbc.
		Select("tracks.*").
		Where("album_id=?", id.Value).
		Order("tracks.tag_disc_number, tracks.tag_track_number").
		Order("filename").
		Find(&childTracks).Error; err != nil {
		return spec.NewError(0, "error finding child tracks: %v", err)
	}
	children, err := spec.TrackChildrenByFolder(rnd, childTracks)
	if err != nil {
		return spec.NewError(0, "render child tracks: %v", err)
	}
	for _, child := range children {
		if v, _ := params.Get("c"); v == "Jamstash" {
			// jamstash thinks it can't play flacs
			child.ContentType = "audio/mpeg"
			child.Suffix = "mp3"
		}
	}
	childrenObj = append(childrenObj, children...)
	// respond section
	sub := spec.NewResponse()
	directories, err := spec.DirectoriesByFolder(rnd, []*db.Album{folder})
	if err != nil {
		return spec.NewError(0, "render directory: %v", err)
	}
	sub.Directory = directories[0]
	sub.Directory.Children = childrenObj
	return sub
}

// ServeGetAlbumList handles the getAlbumList view.
// changes to this function should be reflected in _by_tags.go's
// getAlbumListTwo() function
func (c *Controller) ServeGetAlbumList(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	rnd := render(c, r)
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
		q = q.Where("albums.tag_year BETWEEN ? AND ?", min(y1, y2), max(y1, y2))
		q = q.Order("albums.tag_year DESC")
	case "byGenre":
		genre, _ := params.Get("genre")
		q = q.Joins("JOIN album_genres ON album_genres.album_id=albums.id")
		q = q.Joins("JOIN genres ON genres.id=album_genres.genre_id AND genres.name=?", genre)
		q = q.Order("right_path")
	case "frequent":
		q = q.Scopes(spec.AlbumWithUserPlay(user.ID)).Having("play_length > 0").Order("play_length DESC")
	case "newest":
		q = q.Order("created_at DESC")
	case "random":
		q = q.Order(gorm.Expr("random()"))
	case "recent":
		q = q.Scopes(spec.AlbumWithUserPlay(user.ID)).Having("play_time IS NOT NULL").Order("play_time DESC")
	case "starred":
		q = q.Joins("JOIN album_stars ON albums.id=album_stars.album_id AND album_stars.user_id=?", user.ID)
		q = q.Order("right_path")
	case "highest":
		q = q.Joins("JOIN album_ratings ON album_ratings.album_id=albums.id AND album_ratings.user_id=?", user.ID)
		q = q.Order("album_ratings.rating DESC")
	default:
		return spec.NewError(10, "unknown value %q for parameter 'type'", v)
	}

	q = q.Scopes(spec.WithAlbumRootDir(rnd.MusicFolder))
	var folders []*db.Album
	if err := q.
		Select("albums.*").
		Joins("JOIN album_credits ON album_credits.album_id=albums.id AND album_credits.role=?", db.RoleAlbumArtist).
		Group("albums.id").
		Order("albums.id"). // tiebreak so equal sort values keep a stable order across pages
		Offset(params.GetOrInt("offset", 0)).
		Limit(params.GetOrInt("size", 10)).
		Find(&folders).Error; err != nil {
		return spec.NewError(0, "error finding albums: %v", err)
	}
	albums, err := spec.AlbumsByFolder(rnd, folders)
	if err != nil {
		return spec.NewError(0, "render albums: %v", err)
	}
	sub := spec.NewResponse()
	sub.Albums = &spec.Albums{List: albums}
	return sub
}

func (c *Controller) ServeSearchTwo(r *http.Request) *spec.Response {
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

	results := &spec.SearchResultTwo{}

	// search "artists"
	rootQ := c.dbc.
		Select("id").
		Model(&db.Album{}).
		Where("parent_id IS NULL").
		Scopes(spec.WithAlbumRootDir(rnd.MusicFolder))

	q := c.dbc.
		Select("albums.*").
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
	var artists []*db.Album
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(0, "find artists: %v", err)
	}
	artistDirs, err := spec.DirectoriesByFolder(rnd, artists)
	if err != nil {
		return spec.NewError(0, "render artists: %v", err)
	}
	results.Artists = append(results.Artists, artistDirs...)

	// search "albums"
	q = c.dbc.
		Select("albums.*").
		Scopes(spec.WithAlbumRootDir(rnd.MusicFolder)).
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
	var albums []*db.Album
	if err := q.Find(&albums).Error; err != nil {
		return spec.NewError(0, "find albums: %v", err)
	}
	albumChildren, err := spec.TCAlbumsByFolder(rnd, albums)
	if err != nil {
		return spec.NewError(0, "render albums: %v", err)
	}
	results.Albums = append(results.Albums, albumChildren...)

	// search tracks
	q = c.dbc.
		Select("tracks.*")
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
	if rnd.MusicFolder != "" {
		q = q.
			Joins("JOIN albums ON albums.id=tracks.album_id").
			Scopes(spec.WithAlbumRootDir(rnd.MusicFolder))
	}
	var tracks []*db.Track
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find tracks: %v", err)
	}
	trackChildren, err := spec.TrackChildrenByFolder(rnd, tracks)
	if err != nil {
		return spec.NewError(0, "render tracks: %v", err)
	}
	results.Tracks = append(results.Tracks, trackChildren...)

	sub := spec.NewResponse()
	sub.SearchResultTwo = results
	return sub
}

func (c *Controller) ServeGetArtistInfo(_ *http.Request) *spec.Response {
	return spec.NewResponse()
}

func (c *Controller) ServeGetStarred(r *http.Request) *spec.Response {
	user := r.Context().Value(CtxUser).(*db.User)
	rnd := render(c, r)

	results := &spec.Starred{}

	// "artists"
	rootQ := c.dbc.
		Select("id").
		Model(&db.Album{}).
		Where("parent_id IS NULL").
		Scopes(spec.WithAlbumRootDir(rnd.MusicFolder))

	q := c.dbc.
		Select("albums.*").
		Where(`parent_id IN ?`, rootQ.SubQuery()).
		Joins("JOIN album_stars ON albums.id=album_stars.album_id").
		Where("album_stars.user_id=?", user.ID)
	var artists []*db.Album
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(0, "find artists: %v", err)
	}
	artistDirs, err := spec.DirectoriesByFolder(rnd, artists)
	if err != nil {
		return spec.NewError(0, "render artists: %v", err)
	}
	results.Artists = append(results.Artists, artistDirs...)

	// "albums"
	q = c.dbc.
		Select("albums.*").
		Scopes(spec.WithAlbumRootDir(rnd.MusicFolder)).
		Joins("JOIN album_credits ON album_credits.album_id=albums.id AND album_credits.role=?", db.RoleAlbumArtist).
		Joins("JOIN album_stars ON albums.id=album_stars.album_id").
		Where("album_stars.user_id=?", user.ID)
	var albums []*db.Album
	if err := q.Find(&albums).Error; err != nil {
		return spec.NewError(0, "find albums: %v", err)
	}
	albumChildren, err := spec.TCAlbumsByFolder(rnd, albums)
	if err != nil {
		return spec.NewError(0, "render albums: %v", err)
	}
	results.Albums = append(results.Albums, albumChildren...)

	// tracks
	q = c.dbc.
		Select("tracks.*").
		Joins("JOIN track_stars ON tracks.id=track_stars.track_id").
		Where("track_stars.user_id=?", user.ID)
	if rnd.MusicFolder != "" {
		q = q.
			Joins("JOIN albums ON albums.id=tracks.album_id").
			Scopes(spec.WithAlbumRootDir(rnd.MusicFolder))
	}
	var tracks []*db.Track
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find tracks: %v", err)
	}
	trackChildren, err := spec.TrackChildrenByFolder(rnd, tracks)
	if err != nil {
		return spec.NewError(0, "render tracks: %v", err)
	}
	results.Tracks = append(results.Tracks, trackChildren...)

	sub := spec.NewResponse()
	sub.Starred = results
	return sub
}
