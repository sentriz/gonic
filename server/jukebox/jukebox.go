package jukebox

import (
	"fmt"
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

type strmInfo struct {
	ctrlStrmr beep.Ctrl
	strm      beep.StreamSeekCloser
	format    beep.Format
}

type Jukebox struct {
	playlist  []*db.Track
	musicPath string
	index     int
	playing   bool
	sr        beep.SampleRate
	// used to notify the player to re read the members
	updates chan struct{}
	quit    chan struct{}
	done    chan bool
	info    *strmInfo
	sync.Mutex
}

func New(musicPath string) *Jukebox {
	return &Jukebox{
		musicPath: musicPath,
		sr:        beep.SampleRate(48000),
		updates:   make(chan struct{}),
		done:      make(chan bool),
		quit:      make(chan struct{}),
	}
}

func (j *Jukebox) Listen() error {
	if err := speaker.Init(j.sr, j.sr.N(time.Second/2)); err != nil {
		return fmt.Errorf("initing speaker: %w", err)
	}
	for {
		select {
		case <-j.quit:
			return nil
		case <-j.updates:
			j.doUpdate()
		}
	}
}

func (j *Jukebox) Quit() {
	j.quit <- struct{}{}
}

func (j *Jukebox) doUpdate() {
	var streamer beep.Streamer
	var format beep.Format
	if j.index >= len(j.playlist) {
		j.Lock()
		j.index = 0
		j.playing = false
		j.Unlock()
		return
	}
	j.Lock()
	f, err := os.Open(path.Join(
		j.musicPath,
		j.playlist[j.index].RelPath(),
	))
	j.Unlock()
	if err != nil {
		j.incIndex()
		return
	}
	switch j.playlist[j.index].Ext() {
	case "mp3":
		streamer, format, err = mp3.Decode(f)
	case "flac":
		streamer, format, err = flac.Decode(f)
	default:
		j.incIndex()
		return
	}
	if err != nil {
		j.incIndex()
		return
	}
	if j.playing {
		j.Lock()
		{
			j.info = &strmInfo{}
			j.info.strm = streamer.(beep.StreamSeekCloser)
			j.info.ctrlStrmr.Streamer = beep.Resample(
				4, format.SampleRate,
				j.sr, j.info.strm,
			)
			j.info.format = format
		}
		j.Unlock()
		speaker.Play(beep.Seq(&j.info.ctrlStrmr, beep.Callback(func() {
			j.done <- false
		})))
		if v := <-j.done; v {
			return
		}
		j.Lock()
		j.index++
		if j.index >= len(j.playlist) {
			j.index = 0
			j.playing = false
			j.Unlock()
			return
		}
		j.Unlock()
		// in a go routine as otherwise this hangs as the
		go func() {
			j.updates <- struct{}{}
		}()
	}
}

func (j *Jukebox) incIndex() {
	j.Lock()
	defer j.Unlock()
	j.index++
}

func (j *Jukebox) SetTracks(tracks []*db.Track) {
	j.Lock()
	defer j.Unlock()
	j.index = 0
	if len(tracks) == 0 {
		if j.playing {
			j.done <- true
		}
		j.playing = false
		j.playlist = []*db.Track{}
		speaker.Clear()
		return
	}
	if j.playing {
		j.playlist = tracks
		j.done <- true
		speaker.Clear()
		j.updates <- struct{}{}
		return
	}
	j.playlist = tracks
	j.playing = true
	j.updates <- struct{}{}
}

func (j *Jukebox) AddTracks(tracks []*db.Track) {
	j.Lock()
	j.playlist = append(j.playlist, tracks...)
	j.Unlock()
}

func (j *Jukebox) ClearTracks() {
	j.Lock()
	j.index = 0
	j.playing = false
	j.playlist = []*db.Track{}
	j.Unlock()
}

func (j *Jukebox) RemoveTrack(i int) {
	j.Lock()
	defer j.Unlock()
	if i < 0 || i > len(j.playlist) {
		return
	}
	j.playlist = append(j.playlist[:i], j.playlist[i+1:]...)
}

func (j *Jukebox) Status() *spec.JukeboxStatus {
	position := 0
	if j.info != nil {
		length := j.info.format.SampleRate.D(j.info.strm.Position())
		position = int(length.Round(time.Millisecond).Seconds())
	}
	return &spec.JukeboxStatus{
		CurrentIndex: j.index,
		Playing:      j.playing,
		Gain:         0.9,
		Position:     position,
	}
}

func (j *Jukebox) GetTracks() *spec.JukeboxPlaylist {
	j.Lock()
	defer j.Unlock()
	jb := &spec.JukeboxPlaylist{}
	jb.List = make([]*spec.TrackChild, len(j.playlist))
	for i, track := range j.playlist {
		jb.List[i] = spec.NewTrackByTags(track, track.Album)
	}
	jb.CurrentIndex = j.index
	jb.Playing = j.playing
	jb.Gain = 0.9
	jb.Position = 0
	if j.info != nil {
		length := j.info.format.SampleRate.D(j.info.strm.Position())
		jb.Position = int(length.Round(time.Millisecond).Seconds())
	}
	return jb
}

func (j *Jukebox) Stop() {
	j.Lock()
	j.playing = false
	j.info.ctrlStrmr.Paused = true
	j.Unlock()
}

func (j *Jukebox) Start() {
	j.Lock()
	j.playing = true
	j.info.ctrlStrmr.Paused = false
	j.Unlock()
}

func (j *Jukebox) Skip(i int, skipCurrent bool) {
	j.Lock()
	defer j.Unlock()
	if i == j.index {
		return
	}
	if skipCurrent {
		j.index++
	} else {
		j.index = i
	}
	speaker.Clear()
	if j.playing {
		j.done <- true
	}
	j.updates <- struct{}{}
}
