//nolint:revive
package artistinfocache

import (
	"cmp"
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

	if artistInfo.ID == 0 || time.Since(artistInfo.UpdatedAt) > keepFor {
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

	// lastfm is best-effort. without an api key these calls fail, but we can still resolve
	// similar artists from musicbrainz using the artist's tag-scanned mbid. only overwrite
	// fields when we actually have new data, so a transient failure can't blank the cached row
	info, err := a.lastfmClient.ArtistGetInfo(artist.Name)
	if err != nil {
		log.Printf("error getting lastfm artist info for %q: %v", artist.Name, err)
		// non-fatal
	}

	if info.Bio.Summary != "" {
		artistInfo.Biography = info.Bio.Summary
	}
	if info.URL != "" {
		artistInfo.LastFMURL = info.URL
	}

	mbid := cmp.Or(artist.MusicBrainzID, info.MBID, artistInfo.MusicBrainzID)
	artistInfo.MusicBrainzID = mbid

	var similar []string
	for _, sim := range info.Similar.Artists {
		similar = append(similar, sim.Name)
	}
	if a.mbClient != nil && mbid != "" {
		if related, err := musicBrainzRelatedArtists(ctx, a.mbClient, mbid); err != nil {
			log.Printf("error fetching musicbrainz artist %s: %v", mbid, err)
			// non-fatal
		} else {
			similar = append(similar, related...)
		}
	}
	if len(similar) > 0 {
		artistInfo.SetSimilarArtists(dedupe(similar))
	}

	if info.URL != "" {
		if url, err := a.lastfmClient.StealArtistImage(info.URL); err != nil {
			log.Printf("error stealing lastfm artist image: %v", err)
			// non-fatal
		} else if url != "" {
			artistInfo.ImageURL = url
		}
	}

	if topTracks, err := a.lastfmClient.ArtistGetTopTracks(artist.Name); err != nil {
		log.Printf("error getting lastfm top tracks for %q: %v", artist.Name, err)
		// non-fatal
	} else if len(topTracks.Tracks) > 0 {
		var names []string
		for _, tr := range topTracks.Tracks {
			names = append(names, tr.Name)
		}
		artistInfo.SetTopTracks(names)
	}

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
