//nolint:revive
package albuminfocache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/lastfm"
)

const keepFor = 30 * time.Hour * 24

type AlbumInfoCache struct {
	db           *db.DB
	lastfmClient *lastfm.Client
}

func New(db *db.DB, lastfmClient *lastfm.Client) *AlbumInfoCache {
	return &AlbumInfoCache{db: db, lastfmClient: lastfmClient}
}

func (a *AlbumInfoCache) GetOrLookup(ctx context.Context, albumID int) (*db.AlbumInfo, error) {
	var album db.Album
	if err := a.db.Find(&album, "id=?", albumID).Error; err != nil {
		return nil, fmt.Errorf("find album in db: %w", err)
	}
	if album.TagAlbumArtist == "" || album.TagTitle == "" {
		return nil, fmt.Errorf("no metadata to look up")
	}

	var albumInfo db.AlbumInfo
	if err := a.db.Find(&albumInfo, "id=?", albumID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find album info in db: %w", err)
	}

	if albumInfo.ID == 0 || time.Since(albumInfo.UpdatedAt) > keepFor {
		return a.Lookup(ctx, &album)
	}

	return &albumInfo, nil
}

func (a *AlbumInfoCache) Get(ctx context.Context, albumID int) (*db.AlbumInfo, error) {
	var albumInfo db.AlbumInfo
	if err := a.db.Find(&albumInfo, "id=?", albumID).Error; err != nil {
		return nil, fmt.Errorf("find album info in db: %w", err)
	}
	return &albumInfo, nil
}

func (a *AlbumInfoCache) Lookup(ctx context.Context, album *db.Album) (*db.AlbumInfo, error) {
	var albumInfo db.AlbumInfo
	albumInfo.ID = album.ID

	if err := a.db.FirstOrCreate(&albumInfo, "id=?", albumInfo.ID).Error; err != nil {
		return nil, fmt.Errorf("first or create album info: %w", err)
	}
	if err := a.db.Save(&albumInfo).Error; err != nil {
		return nil, fmt.Errorf("bump updated_at time: %w", err)
	}

	info, err := a.lastfmClient.AlbumGetInfo(album.TagAlbumArtist, album.TagTitle)
	if err != nil {
		return nil, fmt.Errorf("get upstream info: %w", err)
	}

	albumInfo.ID = album.ID
	albumInfo.Notes = info.Wiki.Content
	albumInfo.MusicBrainzID = info.MBID
	albumInfo.LastFMURL = info.URL

	if err := a.db.Save(&albumInfo).Error; err != nil {
		return nil, fmt.Errorf("save upstream info: %w", err)
	}

	return &albumInfo, nil
}
