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
	"go.senan.xyz/gonic/musicbrainz"
)

const keepFor = 30 * time.Hour * 24

type ArtistInfoCache struct {
	db           *db.DB
	lastfmClient *lastfm.Client
	mbClient     *musicbrainz.Client
}

func New(db *db.DB, lastfmClient *lastfm.Client, mbClient *musicbrainz.Client) *ArtistInfoCache {
	return &ArtistInfoCache{db: db, lastfmClient: lastfmClient, mbClient: mbClient}
}

func (a *ArtistInfoCache) GetOrLookup(ctx context.Context, artistID int) (*db.ArtistInfo, error) {
	var artist db.Artist
	if err := a.db.Find(&artist, "id=?", artistID).Error; err != nil {
		return nil, fmt.Errorf("find artist in db: %w", err)
	}
	if artist.Name == "" {
		return nil, fmt.Errorf("no metadata to look up")
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

// TODO: this fails on lastfmClient.ArtistGetInfo if there no lastfm api key.
// but once we scan mb artist IDs from tags, we can do the MB performer look up without lastfm
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
	if a.mbClient != nil && info.MBID != "" {
		if related, err := musicBrainzRelatedArtists(ctx, a.mbClient, info.MBID); err != nil {
			log.Printf("error fetching musicbrainz artist %s: %v", info.MBID, err)
			// non-fatal
		} else {
			similar = append(similar, related...)
		}
	}
	artistInfo.SetSimilarArtists(dedupe(similar))

	url, err := a.lastfmClient.StealArtistImage(info.URL)
	if err != nil {
		log.Printf("error stealing lastfm artist image: %v", err)
		// non-fatal
	}
	if url != "" {
		artistInfo.ImageURL = url
	}

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
	for {
		q := a.db.
			Where("artist_infos.id IS NULL OR artist_infos.updated_at<?", time.Now().Add(-keepFor)).
			Joins("LEFT JOIN artist_infos ON artist_infos.id=artists.id").
			Limit(1)

		var artist db.Artist
		if err := q.Find(&artist).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("finding non cached artist: %w", err)
		}

		if artist.ID == 0 {
			return nil
		}

		if _, err := a.Lookup(context.Background(), &artist); err != nil {
			log.Printf("error looking up non cached artist %q: %v", artist.Name, err)
			continue
		}

		log.Printf("cached artist info for %q", artist.Name)
	}
}

func musicBrainzRelatedArtists(ctx context.Context, mbClient *musicbrainz.Client, mbid string) ([]string, error) {
	seen := map[string]struct{}{mbid: {}}
	return walkMusicBrainzRelatedArtists(ctx, mbClient, mbid, seen, 0)
}

func walkMusicBrainzRelatedArtists(ctx context.Context, mbClient *musicbrainz.Client, mbid string, seen map[string]struct{}, depth int) ([]string, error) {
	const maxDepth = 1

	artist, err := mbClient.GetArtist(ctx, mbid, "artist-rels")
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", mbid, err)
	}

	var out []string
	for _, rel := range artist.Relations {
		if rel.TargetType != "artist" {
			continue
		}
		if rel.TypeID != musicbrainz.LinkTypeIDIsPerson && rel.TypeID != musicbrainz.LinkTypeIDMemberOfBand {
			continue
		}
		target := rel.Artist
		if _, ok := seen[target.ID]; ok {
			continue
		}
		seen[target.ID] = struct{}{}

		out = append(out, target.Name)
		if rel.Direction == musicbrainz.DirectionBackward && depth < maxDepth {
			sub, err := walkMusicBrainzRelatedArtists(ctx, mbClient, target.ID, seen, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		}
	}
	return out, nil
}

func dedupe(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := in[:0]
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
