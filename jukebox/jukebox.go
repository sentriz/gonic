package jukebox

import (
	"os"
	"path"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
)

type Jukebox struct {
	playlist     []*db.Track
	playlistLock *sync.Mutex
	index        int
	playing      bool
	// unimplemented for now
	position int
	// used to notify the player to re read the members
	updates chan struct{}
	done    chan bool
	// used to allow pausing or unpausing music
	ctrlStrmr beep.Ctrl
}

func (j *Jukebox) Init(musicPath string) {
	j.playlistLock = &sync.Mutex{}
	j.updates = make(chan struct{})
	sr := beep.SampleRate(48000)
	speaker.Init(sr, sr.N(time.Second/2))
	j.done = make(chan bool)

	go func() {
		var streamer beep.Streamer
		var format beep.Format
		var resampled *beep.Resampler
		for range j.updates {
			f, err := os.Open(path.Join(musicPath, j.playlist[j.index].RelPath()))
			if err != nil {
				j.index += 1
				continue
			}
			switch j.playlist[j.index].Ext() {
			case "mp3":
				streamer, format, err = mp3.Decode(f)
			case "flac":
				streamer, format, err = flac.Decode(f)
			default:
				j.index += 1
				continue
			}
			if err != nil {
				j.index += 1
				continue
			}
			if j.playing {
				resampled = beep.Resample(4, format.SampleRate, sr, streamer)
				j.ctrlStrmr.Streamer = resampled
				speaker.Play(beep.Seq(&j.ctrlStrmr, beep.Callback(func() {
					j.done <- false
				})))
				if v := <-j.done; !v {
					j.index += 1
					j.playlistLock.Lock()
					if j.index > len(j.playlist) {
						j.index = 0
						j.playing = false
					}
					j.playlistLock.Unlock()
					// in a go routine as otherwise this hangs as the
					go func() {
						j.updates <- struct{}{}
					}()
				} else {
					continue
				}

			}

		}
	}()
}

func (j *Jukebox) SetTracks(tracks []*db.Track) {
	j.playlistLock.Lock()
	j.index = 0
	j.playing = true
	if len(j.playlist) > 0 {
		j.done <- true
		j.playlist = []*db.Track{}
		speaker.Clear()
	}
	j.playlist = tracks
	j.playlistLock.Unlock()
	j.updates <- struct{}{}
}

func (j *Jukebox) AddTracks(tracks []*db.Track) {
	j.playlistLock.Lock()
	j.playlist = append(j.playlist, tracks...)
	j.playlistLock.Unlock()
}

func (j *Jukebox) ClearTracks() {
	j.playlistLock.Lock()
	j.index = 0
	j.playing = false
	j.playlist = []*db.Track{}
	j.playlistLock.Unlock()
}

func (j *Jukebox) RemoveTrack(i int) {
	j.playlistLock.Lock()
	defer j.playlistLock.Unlock()
	if i < 0 || i > len(j.playlist) {
		return
	}
	j.playlist = append(j.playlist[:i], j.playlist[i+1:]...)

}

func (j *Jukebox) Status() *spec.JukeboxStatus {
	return &spec.JukeboxStatus{
		CurrentIndex: j.index,
		Playing:      j.playing,
		Gain:         0.9,
		Position:     j.position,
	}
}

func (j *Jukebox) GetTracks() *spec.JukeboxPlaylist {
	jb := &spec.JukeboxPlaylist{}
	jb.List = make([]*spec.TrackChild, len(j.playlist))
	for i, track := range j.playlist {
		jb.List[i] = spec.NewTrackByTags(track, track.Album)
	}
	jb.CurrentIndex = j.index
	jb.Playing = j.playing
	jb.Gain = 0.9
	jb.Position = j.position
	return jb
}

func (j *Jukebox) Stop() {
	j.playing = false
	j.ctrlStrmr.Paused = true
}

func (j *Jukebox) Start() {
	j.playing = false
	j.ctrlStrmr.Paused = false
}

func (j *Jukebox) Skip() {
	// This isnt fully implemented, we cant skip into a track, and i
	// cant see how to with beep
	j.index += 1
	speaker.Clear()
	j.done <- true
	j.updates <- struct{}{}
}
