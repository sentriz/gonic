//nolint:revive
package artistinfocache

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/jinzhu/gorm"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/lastfm"
)

const keepFor = 30 * time.Hour * 24

type ArtistInfoCache struct {
	db           *db.DB
	lastfmClient *lastfm.Client
}

func New(db *db.DB, lastfmClient *lastfm.Client) *ArtistInfoCache {
	return &ArtistInfoCache{db: db, lastfmClient: lastfmClient}
}

func (a *ArtistInfoCache) getOrLookup(ctx context.Context, artistName string) (*db.ArtistInfo, error) {
	var artistInfo db.ArtistInfo
	if err := a.db.Find(&artistInfo, "name=?", strcase.ToDelimited(artistName, ' ')).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find artist info in db: %w", err)
	}

	if artistInfo.Biography == "" /* prev not found maybe */ || time.Since(artistInfo.UpdatedAt) > keepFor {
		return a.Lookup(ctx, artistName)
	}

	return &artistInfo, nil
}

func (a *ArtistInfoCache) GetOrLookupByArtist(ctx context.Context, artistID int) (*db.ArtistInfo, error) {
	var artist db.Artist
	if err := a.db.Find(&artist, "id=?", artistID).Error; err != nil {
		return nil, fmt.Errorf("find artist in db: %w", err)
	}

	return a.getOrLookup(ctx, artist.Name)
}

func (a *ArtistInfoCache) GetOrLookupByAlbum(ctx context.Context, albumID int) (*db.ArtistInfo, error) {
	var album db.Album
	if err := a.db.Find(&album, "id=?", albumID).Error; err != nil {
		return nil, fmt.Errorf("find artist in db: %w", err)
	}

	return a.getOrLookup(ctx, album.RightPath)
}

func (a *ArtistInfoCache) Get(_ context.Context, artistName string) (*db.ArtistInfo, error) {
	var artistInfo db.ArtistInfo
	if err := a.db.Find(&artistInfo, "name=?", strcase.ToDelimited(artistName, ' ')).Error; err != nil {
		return nil, fmt.Errorf("find artist info in db: %w", err)
	}
	return &artistInfo, nil
}

func (a *ArtistInfoCache) Lookup(_ context.Context, artistName string) (*db.ArtistInfo, error) {
	var artistInfo db.ArtistInfo
	artistInfo.Name = strcase.ToDelimited(artistName, ' ')

	if err := a.db.FirstOrCreate(&artistInfo, "name=?", artistInfo.Name).Error; err != nil {
		return nil, fmt.Errorf("first or create artist info: %w", err)
	}

	info, err := a.lastfmClient.ArtistGetInfo(artistName)
	if err != nil {
		return nil, fmt.Errorf("get upstream info: %w", err)
	}

	artistInfo.Biography = info.Bio.Summary
	artistInfo.MusicBrainzID = info.MBID
	artistInfo.LastFMURL = info.URL

	var similar []string
	for _, sim := range info.Similar.Artists {
		similar = append(similar, sim.Name)
	}
	artistInfo.SetSimilarArtists(similar)

	url, _ := a.lastfmClient.StealArtistImage(info.URL)
	artistInfo.ImageURL = url

	topTracksResponse, err := a.lastfmClient.ArtistGetTopTracks(artistName)
	if err != nil {
		return nil, fmt.Errorf("get top tracks: %w", err)
	}
	var topTracks []string
	for _, tr := range topTracksResponse.Tracks {
		topTracks = append(topTracks, tr.Name)
	}
	artistInfo.SetTopTracks(topTracks)

	if err := a.db.Save(&artistInfo).Error; err != nil {
		return nil, fmt.Errorf("save upstream info: %w", err)
	}

	return &artistInfo, nil
}

func (a *ArtistInfoCache) Refresh() error {
	q := a.db.
		Where("artist_infos.updated_at<?", time.Now().Add(-keepFor))

	var artistInfo db.ArtistInfo
	err := q.Find(&artistInfo).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// No outdated records
		return nil
	}
	if err != nil {
		return fmt.Errorf("finding non cached artist: %w", err)
	}

	if _, err := a.Lookup(context.Background(), artistInfo.Name); err != nil {
		return fmt.Errorf("looking up non cached artist %s: %w", artistInfo.Name, err)
	}

	log.Printf("cached artist info for %q", artistInfo.Name)

	return nil
}
