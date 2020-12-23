package ctrladmin

import (
	"bufio"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"go.senan.xyz/gonic/server/db"
)

var (
	errPlaylistNoMatch = errors.New("couldn't match track")
)

func playlistParseLine(c *Controller, path string) (int, error) {
	if strings.HasPrefix(path, "#") || strings.TrimSpace(path) == "" {
		return 0, nil
	}
	var track db.Track
	query := c.DB.Raw(`
		SELECT tracks.id FROM TRACKS
		JOIN albums ON tracks.album_id=albums.id
		WHERE (? || '/' || albums.left_path || albums.right_path || '/' || tracks.filename)=?`,
		c.MusicPath, path)
	err := query.First(&track).Error
	switch {
	case gorm.IsRecordNotFoundError(err):
		return 0, fmt.Errorf("%v: %w", err, errPlaylistNoMatch)
	case err != nil:
		return 0, fmt.Errorf("while matching: %w", err)
	default:
		return track.ID, nil
	}
}

func playlistCheckContentType(contentType string) bool {
	contentType = strings.ToLower(contentType)
	known := map[string]struct{}{
		"audio/x-mpegurl":          {},
		"audio/mpegurl":            {},
		"application/octet-stream": {},
	}
	_, ok := known[contentType]
	return ok
}

func playlistParseUpload(c *Controller, userID int, header *multipart.FileHeader) ([]string, bool) {
	file, err := header.Open()
	if err != nil {
		return []string{fmt.Sprintf("couldn't open file %q", header.Filename)}, false
	}
	playlistName := strings.TrimSuffix(header.Filename, ".m3u8")
	if playlistName == "" {
		return []string{fmt.Sprintf("invalid filename %q", header.Filename)}, false
	}
	contentType := header.Header.Get("Content-Type")
	if !playlistCheckContentType(contentType) {
		return []string{fmt.Sprintf("invalid content-type %q", contentType)}, false
	}
	var trackIDs []int
	var errors []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		trackID, err := playlistParseLine(c, scanner.Text())
		if err != nil {
			// trim length of error to not overflow cookie flash
			errors = append(errors, fmt.Sprintf("%.100s", err.Error()))
		}
		if trackID != 0 {
			trackIDs = append(trackIDs, trackID)
		}
	}
	if err := scanner.Err(); err != nil {
		return []string{fmt.Sprintf("iterating playlist file: %v", err)}, true
	}
	playlist := &db.Playlist{}
	c.DB.FirstOrCreate(playlist, db.Playlist{
		Name:   playlistName,
		UserID: userID,
	})
	playlist.SetItems(trackIDs)
	c.DB.Save(playlist)
	return errors, true
}

func (c *Controller) ServeUploadPlaylist(r *http.Request) *Response {
	return &Response{template: "upload_playlist.tmpl"}
}

func (c *Controller) ServeUploadPlaylistDo(r *http.Request) *Response {
	if err := r.ParseMultipartForm((1 << 10) * 24); err != nil {
		return &Response{
			err:  "couldn't parse mutlipart",
			code: 500,
		}
	}
	user := r.Context().Value(CtxUser).(*db.User)
	var playlistCount int
	var errors []string
	for _, headers := range r.MultipartForm.File {
		for _, header := range headers {
			headerErrors, created := playlistParseUpload(c, user.ID, header)
			if created {
				playlistCount++
			}
			errors = append(errors, headerErrors...)
		}
	}
	return &Response{
		redirect: "/admin/home",
		flashN:   []string{fmt.Sprintf("%d playlist(s) created", playlistCount)},
		flashW:   errors,
	}
}

func (c *Controller) ServeDeletePlaylistDo(r *http.Request) *Response {
	user := r.Context().Value(CtxUser).(*db.User)
	id, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		return &Response{
			err:  "please provide a valid id",
			code: 400,
		}
	}
	c.DB.
		Where("user_id=? AND id=?", user.ID, id).
		Delete(db.Playlist{})
	return &Response{
		redirect: "/admin/home",
	}
}
