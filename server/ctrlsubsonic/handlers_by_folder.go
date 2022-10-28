package ctrlsubsonic

import (
	"fmt"
	"errors"
	"net/http"
	"strings"

	"github.com/jinzhu/gorm"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/params"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/transcode"
)

// the subsonic spec mentions "artist" a lot when talking about the
// browse by folder endpoints. but since we're not browsing by tag
// we can't access artists. so instead we'll consider the artist of
// an track to be the it's respective folder that comes directly
// under the root directory

func (c *Controller) ServeGetIndexes(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	rootQ := c.DB.
		Select("id").
		Model(&db.Album{}).
		Where("parent_id IS NULL")
	if m := c.getMusicFolder(params); m != "" {
		rootQ = rootQ.
			Where("root_dir=?", m)
	}
	var folders []*db.Album
	c.DB.
		Select("*, count(sub.id) child_count").
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID).
		Joins("LEFT JOIN albums sub ON albums.id=sub.parent_id").
		Where("albums.parent_id IN ?", rootQ.SubQuery()).
		Group("albums.id").
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
	c.DB.
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID).
		First(folder, id.Value)
	// start looking for child childFolders in the current dir
	var childFolders []*db.Album
	c.DB.
		Where("parent_id=?", id.Value).
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID).
		Order("albums.right_path COLLATE NOCASE").
		Find(&childFolders)
	for _, ch := range childFolders {
		childrenObj = append(childrenObj, spec.NewTCAlbumByFolder(ch))
	}
	// start looking for child childTracks in the current dir
	var childTracks []*db.Track
	c.DB.
		Where("album_id=?", id.Value).
		Preload("Album").
		Preload("Album.TagArtist").
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID).
		Order("filename").
		Find(&childTracks)

	//Get the transcoder profile to serve the transcoded MIME type and Suffix
	var transcodeMIME = ""
	var transcodeSuffix = ""
	var transcodeOk = false
	pref, err := streamGetTransPref(c.DB, user.ID, params.GetOr("c", ""))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
	        return spec.NewError(0, "couldn't find transcode preference: %v", err)
	} else {
	        profile, ok := transcode.UserProfiles[pref.Profile]
	        if ok {
	                transcodeOk = true
	                transcodeMIME = profile.MIME()
	                transcodeSuffix = profile.Suffix()
	        }
	}

	for _, ch := range childTracks {
		toAppend := spec.NewTCTrackByFolder(ch, folder)
		if v, _ := params.Get("c"); v == "Jamstash" {
			// jamstash thinks it can't play flacs
			toAppend.ContentType = "audio/mpeg"
			toAppend.Suffix = "mp3"
		}
		if transcodeOk {
			toAppend.TranscodedContentType = transcodeMIME
			toAppend.TranscodedSuffix = transcodeSuffix
		}
		childrenObj = append(childrenObj, toAppend)
	}
	// respond section
	sub := spec.NewResponse()
	sub.Directory = spec.NewDirectoryByFolder(folder, childrenObj)
	return sub
}

// ServeGetAlbumList handles the getAlbumList view.
// changes to this function should be reflected in in _by_tags.go's
// getAlbumListTwo() function
func (c *Controller) ServeGetAlbumList(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)
	q := c.DB.DB
	switch v, _ := params.Get("type"); v {
	case "alphabeticalByArtist":
		q = q.Joins(`
			JOIN albums parent_albums
			ON albums.parent_id=parent_albums.id`)
		q = q.Order("parent_albums.right_path")
	case "alphabeticalByName":
		q = q.Order("right_path")
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
		q = q.Order("right_path")
	case "frequent":
		q = q.Joins(`
			JOIN plays
			ON albums.id=plays.album_id AND plays.user_id=?`,
			user.ID)
		q = q.Order("plays.count DESC")
	case "newest":
		q = q.Order("created_at DESC")
	case "random":
		q = q.Order(gorm.Expr("random()"))
	case "recent":
		q = q.Joins(`
			JOIN plays
			ON albums.id=plays.album_id AND plays.user_id=?`,
			user.ID)
		q = q.Order("plays.time DESC")
	case "starred":
		q = q.Joins("JOIN album_stars ON albums.id=album_stars.album_id AND album_stars.user_id=?", user.ID)
		q = q.Order("right_path")
	default:
		return spec.NewError(10, "unknown value `%s` for parameter 'type'", v)
	}

	if m := c.getMusicFolder(params); m != "" {
		q = q.Where("root_dir=?", m)
	}
	var folders []*db.Album
	// TODO: think about removing this extra join to count number
	// of children. it might make sense to store that in the db
	q.
		Select("albums.*, count(tracks.id) child_count, sum(tracks.length) duration").
		Joins("LEFT JOIN tracks ON tracks.album_id=albums.id").
		Group("albums.id").
		Where("albums.tag_artist_id IS NOT NULL").
		Offset(params.GetOrInt("offset", 0)).
		Limit(params.GetOrInt("size", 10)).
		Preload("Parent").
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID).
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
	query = fmt.Sprintf("%%%s%%", strings.Trim(query, `*"'`))

	results := &spec.SearchResultTwo{}

	// search "artists"
	rootQ := c.DB.
		Select("id").
		Model(&db.Album{}).
		Where("parent_id IS NULL")
	if m := c.getMusicFolder(params); m != "" {
		rootQ = rootQ.Where("root_dir=?", m)
	}

	var artists []*db.Album
	q := c.DB.
		Where(`parent_id IN ? AND (right_path LIKE ? OR right_path_u_dec LIKE ?)`, rootQ.SubQuery(), query, query).
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID).
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
	q = c.DB.
		Where(`tag_artist_id IS NOT NULL AND (right_path LIKE ? OR right_path_u_dec LIKE ?)`, query, query).
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID).
		Offset(params.GetOrInt("albumOffset", 0)).
		Limit(params.GetOrInt("albumCount", 20))
	if m := c.getMusicFolder(params); m != "" {
		q = q.Where("root_dir=?", m)
	}
	if err := q.Find(&albums).Error; err != nil {
		return spec.NewError(0, "find albums: %v", err)
	}
	for _, a := range albums {
		results.Albums = append(results.Albums, spec.NewTCAlbumByFolder(a))
	}

	// search tracks
	var tracks []*db.Track
	q = c.DB.
		Preload("Album").
		Where("filename LIKE ? OR filename_u_dec LIKE ?", query, query).
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID).
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

	//Get the transcoder profile to serve the transcoded MIME type and Suffix
	var transcodeMIME = ""
	var transcodeSuffix = ""
	var transcodeOk = false
	pref, err := streamGetTransPref(c.DB, user.ID, params.GetOr("c", ""))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
	        return spec.NewError(0, "couldn't find transcode preference: %v", err)
	} else {
	        profile, ok := transcode.UserProfiles[pref.Profile]
	        if ok {
	                transcodeOk = true
	                transcodeMIME = profile.MIME()
	                transcodeSuffix = profile.Suffix()
	        }
	}

	for _, t := range tracks {
		var track = spec.NewTCTrackByFolder(t, t.Album)
		if transcodeOk {
			track.TranscodedContentType = transcodeMIME
			track.TranscodedSuffix = transcodeSuffix
		}
		results.Tracks = append(results.Tracks, track)
	}

	sub := spec.NewResponse()
	sub.SearchResultTwo = results
	return sub
}

func (c *Controller) ServeGetArtistInfo(r *http.Request) *spec.Response {
	return spec.NewResponse()
}

func (c *Controller) ServeGetStarred(r *http.Request) *spec.Response {
	params := r.Context().Value(CtxParams).(params.Params)
	user := r.Context().Value(CtxUser).(*db.User)

	results := &spec.Starred{}

	// "artists"
	rootQ := c.DB.
		Select("id").
		Model(&db.Album{}).
		Where("parent_id IS NULL")
	if m := c.getMusicFolder(params); m != "" {
		rootQ = rootQ.Where("root_dir=?", m)
	}

	var artists []*db.Album
	q := c.DB.
		Where(`parent_id IN ?`, rootQ.SubQuery()).
		Joins("JOIN album_stars ON albums.id=album_stars.album_id").
		Where("album_stars.user_id=?", user.ID).
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID)
	if err := q.Find(&artists).Error; err != nil {
		return spec.NewError(0, "find artists: %v", err)
	}
	for _, a := range artists {
		results.Artists = append(results.Artists, spec.NewDirectoryByFolder(a, nil))
	}

	// "albums"
	var albums []*db.Album
	q = c.DB.
		Where("tag_artist_id IS NOT NULL").
		Joins("JOIN album_stars ON albums.id=album_stars.album_id").
		Where("album_stars.user_id=?", user.ID).
		Preload("AlbumStar", "user_id=?", user.ID).
		Preload("AlbumRating", "user_id=?", user.ID)
	if m := c.getMusicFolder(params); m != "" {
		q = q.Where("root_dir=?", m)
	}
	if err := q.Find(&albums).Error; err != nil {
		return spec.NewError(0, "find albums: %v", err)
	}
	for _, a := range albums {
		results.Albums = append(results.Albums, spec.NewTCAlbumByFolder(a))
	}

	// tracks
	var tracks []*db.Track
	q = c.DB.
		Preload("Album").
		Joins("JOIN track_stars ON tracks.id=track_stars.track_id").
		Where("track_stars.user_id=?", user.ID).
		Preload("TrackStar", "user_id=?", user.ID).
		Preload("TrackRating", "user_id=?", user.ID)
	if m := c.getMusicFolder(params); m != "" {
		q = q.
			Joins("JOIN albums ON albums.id=tracks.album_id").
			Where("albums.root_dir=?", m)
	}
	if err := q.Find(&tracks).Error; err != nil {
		return spec.NewError(0, "find tracks: %v", err)
	}

	//Get the transcoder profile to serve the transcoded MIME type and Suffix
	var transcodeMIME = ""
	var transcodeSuffix = ""
	var transcodeOk = false
	pref, err := streamGetTransPref(c.DB, user.ID, params.GetOr("c", ""))
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
	        return spec.NewError(0, "couldn't find transcode preference: %v", err)
	} else {
	        profile, ok := transcode.UserProfiles[pref.Profile]
	        if ok {
	                transcodeOk = true
	                transcodeMIME = profile.MIME()
	                transcodeSuffix = profile.Suffix()
	        }
	}

	for _, t := range tracks {
		var track = spec.NewTCTrackByFolder(t, t.Album)
		if transcodeOk {
			track.TranscodedContentType = transcodeMIME
			track.TranscodedSuffix = transcodeSuffix
		}
		results.Tracks = append(results.Tracks, track)
	}

	sub := spec.NewResponse()
	sub.Starred = results
	return sub
}
