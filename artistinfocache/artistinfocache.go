//nolint:revive
package artistinfocache

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

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

func (a *ArtistInfoCache) GetOrLookup(ctx context.Context, artistID int) (*db.ArtistInfo, error) {
	var artist db.Artist
	if err := a.db.Find(&artist, "id=?", artistID).Error; err != nil {
		return nil, fmt.Errorf("find artist in db: %w", err)
	}

	var artistInfo db.ArtistInfo
	if err := a.db.Find(&artistInfo, "id=?", artistID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("find artist info in db: %w", err)
	}

	if artistInfo.ID == 0 || artistInfo.Biography == "" /* prev not found maybe */ || time.Since(artistInfo.UpdatedAt) > keepFor {
		return a.Lookup(ctx, &artist)
	}

	return &artistInfo, nil
}

func (a *ArtistInfoCache) Get(ctx context.Context, artistID int) (*db.ArtistInfo, error) {
	var artistInfo db.ArtistInfo
	if err := a.db.Find(&artistInfo, "id=?", artistID).Error; err != nil {
		return nil, fmt.Errorf("find artist info in db: %w", err)
	}
	return &artistInfo, nil
}

func (a *ArtistInfoCache) Lookup(ctx context.Context, artist *db.Artist) (*db.ArtistInfo, error) {
	var artistInfo db.ArtistInfo
	artistInfo.ID = artist.ID

	if err := a.db.FirstOrCreate(&artistInfo, "id=?", artistInfo.ID).Error; err != nil {
		return nil, fmt.Errorf("first or create artist info: %w", err)
	}
	if err := a.db.Save(&artistInfo).Error; err != nil {
		return nil, fmt.Errorf("bump updated_at time: %w", err)
	}

	info, err := a.lastfmClient.ArtistGetInfo(artist.Name)
	if err != nil {
		return nil, fmt.Errorf("get upstream info: %w", err)
	}

	artistInfo.ID = artist.ID
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

	topTracksResponse, err := a.lastfmClient.ArtistGetTopTracks(artist.Name)
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
		Where("artist_infos.id IS NULL OR artist_infos.updated_at<?", time.Now().Add(-keepFor)).
		Joins("LEFT JOIN artist_infos ON artist_infos.id=artists.id")

	var artist db.Artist
	if err := q.Find(&artist).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("finding non cached artist: %w", err)
	}
	if artist.ID == 0 {
		return nil
	}

	if _, err := a.Lookup(context.Background(), &artist); err != nil {
		return fmt.Errorf("looking up non cached artist %s: %w", artist.Name, err)
	}

	log.Printf("cached artist info for %q", artist.Name)

	return nil
}
