//nolint:revive
package albuminfocache

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"github.com/jinzhu/gorm"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/lastfm"
	"go.senan.xyz/gonic/musicbrainz"
)

const keepFor = 30 * time.Hour * 24

type AlbumInfoCache struct {
	db           *db.DB
	lastfmClient *lastfm.Client
	mbClient     *musicbrainz.Client
}

func New(db *db.DB, lastfmClient *lastfm.Client, mbClient *musicbrainz.Client) *AlbumInfoCache {
	return &AlbumInfoCache{db: db, lastfmClient: lastfmClient, mbClient: mbClient}
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
	if err := a.db.Find(&albumInfo, "id=?", album.ID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find album info: %w", err)
	}
	albumInfo.ID = album.ID

	// lastfm is best-effort. without an api key these calls fail, but we can still resolve
	// musicbrainz facts from the album's tag-scanned mbid. only overwrite fields when we
	// actually have new data, so a transient failure can't blank the cached row
	info, err := a.lastfmClient.AlbumGetInfo(album.TagAlbumArtist, album.TagTitle)
	if err != nil {
		log.Printf("error getting lastfm album info for %q: %v", album.TagTitle, err)
		// non-fatal
	}

	if info.Wiki.Content != "" {
		albumInfo.LastFMNotes = info.Wiki.Content
	}
	if info.URL != "" {
		albumInfo.LastFMURL = info.URL
	}

	mbid := cmp.Or(album.TagBrainzID, info.MBID, albumInfo.MusicBrainzID)
	albumInfo.MusicBrainzID = mbid

	if release := musicBrainzRelease(ctx, a.mbClient, mbid); release != nil {
		var parts []string
		if release.ReleaseGroup != nil && release.ReleaseGroup.Disambiguation != "" {
			parts = append(parts, release.ReleaseGroup.Disambiguation)
		}
		if release.Disambiguation != "" {
			parts = append(parts, release.Disambiguation)
		}

		albumInfo.MusicBrainzDisambiguation = ""
		if len(parts) > 0 {
			albumInfo.MusicBrainzDisambiguation = strings.Join(parts, ", ")
		}
	}

	if err := a.db.Save(&albumInfo).Error; err != nil {
		return nil, fmt.Errorf("save upstream info: %w", err)
	}

	return &albumInfo, nil
}

func Notes(info *db.AlbumInfo) string {
	lastFMNotes := lastfm.CleanText(info.LastFMNotes)

	var mbInfo string
	if d := info.MusicBrainzDisambiguation; d != "" {
		mbInfo = upperFirst(d) + "."
	}

	switch {
	case lastFMNotes != "" && mbInfo != "":
		return lastFMNotes + "\n\n" + mbInfo
	case mbInfo != "":
		return mbInfo
	default:
		return lastFMNotes
	}
}

func musicBrainzRelease(ctx context.Context, mbClient *musicbrainz.Client, mbid string) *musicbrainz.Release {
	if mbClient == nil || mbid == "" {
		return nil
	}

	release, err := mbClient.GetRelease(ctx, mbid, "release-groups")
	if err != nil {
		log.Printf("error fetching musicbrainz release %s: %v", mbid, err)
		return nil
	}
	return release
}

func upperFirst(s string) string {
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
