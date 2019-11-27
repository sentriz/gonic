package ctrladmin

import (
	"bufio"
	"fmt"
	"mime/multipart"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/pkg/errors"

	"senan.xyz/g/gonic/model"
)

func playlistParseLine(c *Controller, playlistID int, path string) error {
	if strings.HasPrefix(path, "#") || strings.TrimSpace(path) == "" {
		return nil
	}
	track := &model.Track{}
	query := c.DB.Raw(`
		SELECT tracks.id FROM TRACKS
		JOIN albums ON tracks.album_id = albums.id
		WHERE (? || '/' || albums.left_path || albums.right_path || '/' || tracks.filename) = ?
	`, c.MusicPath, path)
	err := query.First(&track).Error
	switch {
	case gorm.IsRecordNotFoundError(err):
		return fmt.Errorf("couldn't match track %q", path)
	case err != nil:
		return errors.Wrap(err, "while matching")
	}
	c.DB.Create(&model.PlaylistItem{
		PlaylistID: playlistID,
		TrackID:    track.ID,
	})
	return nil
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
	playlist := &model.Playlist{}
	c.DB.FirstOrCreate(playlist, model.Playlist{
		Name:   playlistName,
		UserID: userID,
	})
	c.DB.Delete(&model.PlaylistItem{}, "playlist_id = ?", playlist.ID)
	var errors []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		path := scanner.Text()
		if err := playlistParseLine(c, playlist.ID, path); err != nil {
			errors = append(errors, err.Error())
		}
	}
	if err := scanner.Err(); err != nil {
		return []string{fmt.Sprintf("scanning line of playlist: %v", err)}, true
	}
	return errors, true
}
