//nolint:revive
package artistinfocache

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
		artistInfo.LastFMBiography = info.Bio.Summary
	}
	if info.URL != "" {
		artistInfo.LastFMURL = info.URL
	}

	mbid := cmp.Or(artist.MusicBrainzID, info.MBID, artistInfo.MusicBrainzID)

	artistInfo.MusicBrainzID = mbid

	var lastFMSimilar []string
	for _, sim := range info.Similar.Artists {
		lastFMSimilar = append(lastFMSimilar, sim.Name)
	}
	if len(lastFMSimilar) > 0 {
		artistInfo.SetLastFMSimilarArtists(lastFMSimilar)
	}

	if mbArtist := musicBrainzArtist(ctx, a.mbClient, mbid); mbArtist != nil {
		setMusicBrainzInfo(&artistInfo, mbArtist)

		related, err := musicBrainzRelatedArtists(ctx, a.mbClient, mbArtist)
		if err != nil {
			log.Printf("error fetching musicbrainz related artists for %s: %v", mbid, err)
			// non-fatal
		}
		if len(related) > 0 {
			artistInfo.SetMusicBrainzRelatedArtists(related)
		}
	}

	if image := lastFMArtistImage(a.lastfmClient, info.URL); image != "" {
		artistInfo.ImageURL = image
	}

	if tracks := lastFMTopTracks(a.lastfmClient, artist.Name); len(tracks) > 0 {
		artistInfo.SetLastFMTopTracks(tracks)
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

func Biography(info *db.ArtistInfo) string {
	var mbParts []string
	if d := info.MusicBrainzDisambiguation; d != "" {
		mbParts = append(mbParts, upperFirst(d))
	}
	if info.MusicBrainzArea != "" {
		mbParts = append(mbParts, "from "+info.MusicBrainzArea)
	}
	if span := lifeSpan(info); span != "" {
		mbParts = append(mbParts, span)
	}

	var mbInfo string
	if len(mbParts) > 0 {
		mbInfo = strings.Join(mbParts, ", ") + "."
	}

	lastFMBio := lastfm.CleanArtistBiography(info.LastFMBiography)

	switch {
	case mbInfo != "" && lastFMBio != "":
		return mbInfo + "\n\n" + lastFMBio
	case mbInfo != "":
		return mbInfo
	default:
		return lastFMBio
	}
}

func lastFMArtistImage(client *lastfm.Client, url string) string {
	if url == "" {
		return ""
	}
	image, err := client.StealArtistImage(url)
	if err != nil {
		log.Printf("error stealing lastfm artist image: %v", err)
		return ""
	}
	return image
}

func lastFMTopTracks(client *lastfm.Client, name string) []string {
	topTracks, err := client.ArtistGetTopTracks(name)
	if err != nil {
		log.Printf("error getting lastfm top tracks for %q: %v", name, err)
		return nil
	}

	names := make([]string, 0, len(topTracks.Tracks))
	for _, tr := range topTracks.Tracks {
		names = append(names, tr.Name)
	}
	return names
}

func musicBrainzArtist(ctx context.Context, mbClient *musicbrainz.Client, mbid string) *musicbrainz.Artist {
	if mbClient == nil || mbid == "" {
		return nil
	}
	artist, err := mbClient.GetArtist(ctx, mbid, "artist-rels")
	if err != nil {
		log.Printf("error fetching musicbrainz artist %s: %v", mbid, err)
		return nil
	}
	return artist
}

func musicBrainzRelatedArtists(ctx context.Context, mbClient *musicbrainz.Client, artist *musicbrainz.Artist) ([]string, error) {
	seen := map[string]struct{}{artist.ID: {}}
	return walkMusicBrainzRelatedArtists(ctx, mbClient, artist, seen, 0)
}

func walkMusicBrainzRelatedArtists(ctx context.Context, mbClient *musicbrainz.Client, artist *musicbrainz.Artist, seen map[string]struct{}, depth int) ([]string, error) {
	const maxDepth = 1

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
			sub, err := mbClient.GetArtist(ctx, target.ID, "artist-rels")
			if err != nil {
				return nil, fmt.Errorf("fetch %s: %w", target.ID, err)
			}
			rels, err := walkMusicBrainzRelatedArtists(ctx, mbClient, sub, seen, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, rels...)
		}
	}
	return out, nil
}

func setMusicBrainzInfo(info *db.ArtistInfo, artist *musicbrainz.Artist) {
	info.MusicBrainzType = artist.Type
	info.MusicBrainzDisambiguation = artist.Disambiguation
	info.MusicBrainzBeginDate = artist.LifeSpan.Begin

	info.MusicBrainzEndDate = ""
	if artist.LifeSpan.Ended {
		info.MusicBrainzEndDate = artist.LifeSpan.End
	}

	info.MusicBrainzArea = ""
	switch {
	case artist.BeginArea != nil:
		info.MusicBrainzArea = artist.BeginArea.Name
	case artist.Area != nil:
		info.MusicBrainzArea = artist.Area.Name
	}
}

func lifeSpan(info *db.ArtistInfo) string {
	begin, end := year(info.MusicBrainzBeginDate), year(info.MusicBrainzEndDate)
	person := info.MusicBrainzType == "Person"
	switch {
	case person && begin != "" && end != "":
		return "born " + begin + ", died " + end
	case person && begin != "":
		return "born " + begin
	case person && end != "":
		return "died " + end
	case begin != "" && end != "":
		return "active " + begin + "–" + end
	case begin != "":
		return "formed " + begin
	case end != "":
		return "disbanded " + end
	default:
		return ""
	}
}

func year(date string) string {
	if len(date) < 4 {
		return ""
	}
	return date[:4]
}

func upperFirst(s string) string {
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
