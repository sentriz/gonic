package ctrladmin

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/scanner"
)

func playlistCheckContentType(contentType string) bool {
	switch ct := strings.ToLower(contentType); ct {
	case
		"audio/x-mpegurl",
		"audio/mpegurl",
		"application/x-mpegurl",
		"application/octet-stream":
		return true
	}
	return false
}

func playlistParseUpload(c *Controller, userID int, header *multipart.FileHeader) ([]string, bool) {
	file, err := header.Open()
	if err != nil {
		return []string{fmt.Sprintf("couldn't open file %q", header.Filename)}, false
	}
	defer file.Close()
	contentType := header.Header.Get("Content-Type")
	if !playlistCheckContentType(contentType) {
		return []string{fmt.Sprintf("invalid content-type %q", contentType)}, false
	}
	return scanner.PlaylistParse(c.DB, userID, header.Filename, file)
}

func (c *Controller) ServeUploadPlaylist(r *http.Request) *Response {
	return &Response{template: "upload_playlist.tmpl"}
}

func (c *Controller) ServeUploadPlaylistDo(r *http.Request) *Response {
	if err := r.ParseMultipartForm((1 << 10) * 24); err != nil {
		return &Response{code: 500, err: "couldn't parse mutlipart"}
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
		return &Response{code: 400, err: "please provide a valid id"}
	}
	c.DB.
		Where("user_id=? AND id=?", user.ID, id).
		Delete(db.Playlist{})
	return &Response{
		redirect: "/admin/home",
	}
}
