package spec

import (
	"fmt"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

// A few endpoints answer with a list the user built rather than one the library
// implies: a play queue, a playlist, bookmarks, the jukebox. Those lists mix
// tracks with podcast episodes and internet radio stations, and have an order
// of their own, so they are rendered from ids rather than from rows.
//
// An entry can outlive what it points at, eg. when files are moved, so a
// missing one renders as nil and the caller decides whether to skip it or say
// something. The result lines up with the ids given either way.

// EntriesByTags renders a mixed list of entries, naming every artist credited.
func EntriesByTags(r Render, ids []specid.ID) ([]*TrackChild, error) {
	tracks, rows, err := findEntries(r, ids)
	if err != nil {
		return nil, err
	}
	children, err := TrackChildrenByTags(r, tracks)
	if err != nil {
		return nil, fmt.Errorf("render tracks: %w", err)
	}
	return entriesInOrder(r, ids, tracks, children, rows), nil
}

// EntriesByFolder renders a mixed list of entries, naming only the artists who
// performed them.
func EntriesByFolder(r Render, ids []specid.ID) ([]*TrackChild, error) {
	tracks, rows, err := findEntries(r, ids)
	if err != nil {
		return nil, err
	}
	children, err := TrackChildrenByFolder(r, tracks)
	if err != nil {
		return nil, fmt.Errorf("render tracks: %w", err)
	}
	return entriesInOrder(r, ids, tracks, children, rows), nil
}

// findEntries fetches both kinds of row an entry can point at.
func findEntries(r Render, ids []specid.ID) ([]*db.Track, entryRows, error) {
	var trackIDs, episodeIDs, stationIDs []int
	for _, id := range ids {
		switch id.Type {
		case specid.Track:
			trackIDs = append(trackIDs, id.Value)
		case specid.PodcastEpisode:
			episodeIDs = append(episodeIDs, id.Value)
		case specid.InternetRadioStation:
			stationIDs = append(stationIDs, id.Value)
		}
	}
	tracks, err := db.FindIn[db.Track](r.DB.DB, "tracks.id", trackIDs)
	if err != nil {
		return nil, entryRows{}, fmt.Errorf("find tracks: %w", err)
	}
	var rows entryRows
	rows.episodes, err = db.FindByID(r.DB.Preload("Podcast"), "id", episodeIDs,
		func(pe *db.PodcastEpisode) int { return pe.ID })
	if err != nil {
		return nil, entryRows{}, fmt.Errorf("find podcast episodes: %w", err)
	}
	rows.stations, err = db.FindByID(r.DB.DB, "id", stationIDs,
		func(irs *db.InternetRadioStation) int { return irs.ID })
	if err != nil {
		return nil, entryRows{}, fmt.Errorf("find internet radio stations: %w", err)
	}
	return tracks, rows, nil
}

// entryRows are the kinds an entry can point at that render on their own, as
// opposed to tracks, which are rendered in a batch that loads for them.
type entryRows struct {
	episodes map[int]*db.PodcastEpisode
	stations map[int]*db.InternetRadioStation
}

// entriesInOrder puts the rendered rows back into the order the entries were
// given in, since that order is the user's and the fetches do not keep it.
func entriesInOrder(r Render, ids []specid.ID, tracks []*db.Track, children []*TrackChild, rows entryRows) []*TrackChild {
	byTrack := make(map[int]*TrackChild, len(tracks))
	for i, track := range tracks {
		byTrack[track.ID] = children[i]
	}
	out := make([]*TrackChild, 0, len(ids))
	for _, id := range ids {
		var child *TrackChild
		switch id.Type {
		case specid.Track:
			child = byTrack[id.Value]
		case specid.PodcastEpisode:
			if episode, ok := rows.episodes[id.Value]; ok {
				child = NewTCPodcastEpisode(episode)
				child.TranscodeMeta = r.TranscodeMeta
			}
		case specid.InternetRadioStation:
			if station, ok := rows.stations[id.Value]; ok {
				child = NewTCInternetRadioStation(station)
			}
		}
		out = append(out, child)
	}
	return out
}
