// author: AlexKraak (https://github.com/alexkraak/)

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
	updates chan update
	quit    chan struct{}
	done    chan bool
	info    *strmInfo
	speaker chan updateSpeaker
	sync.Mutex
}

type updateType string

const (
	set    updateType = "set"
	clear  updateType = "clear"
	skip   updateType = "skip"
	add    updateType = "add"
	remove updateType = "remove"
	stop   updateType = "stop"
	start  updateType = "start"
)

type update struct {
	action updateType
	index  int
	tracks []*db.Track
}

type updateSpeaker struct {
	index int
}

func New(musicPath string) *Jukebox {
	return &Jukebox{
		musicPath: musicPath,
		sr:        beep.SampleRate(48000),
		updates:   make(chan update),
		speaker:   make(chan updateSpeaker, 1),
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
		case update := <-j.updates:
			j.doUpdate(update)
		case speaker := <-j.speaker:
			j.doUpdateSpeaker(speaker)
		}
	}
}

func (j *Jukebox) Quit() {
	j.quit <- struct{}{}
}

func (j *Jukebox) doUpdate(u update) {
	j.Lock()
	switch u.action {
	case set:
		j.playlist = u.tracks
		j.Unlock()
	case clear:
		speaker.Clear()
		j.playing = false
		j.playlist = []*db.Track{}
		j.Unlock()
	case skip:
		speaker.Clear()
		j.index = u.index
		j.playing = true
		j.Unlock()
		j.speaker <- updateSpeaker{j.index}
	case add:
		if len(j.playlist) == 0 {
			j.playlist = u.tracks
			j.playing = true
			j.index = 0
			j.Unlock()
			j.speaker <- updateSpeaker{0}
			return
		}
		j.playlist = append(j.playlist, u.tracks...)
		j.Unlock()
	case remove:
		if u.index < 0 || u.index >= len(j.playlist) {
			j.Unlock()
			return
		}
		j.playlist = append(j.playlist[:u.index], j.playlist[u.index+1:]...)
		j.Unlock()
	case stop:
		if j.info != nil {
			j.playing = false
			j.info.ctrlStrmr.Paused = true
		}
		j.Unlock()
	case start:
		if j.info != nil {
			j.playing = true
			j.info.ctrlStrmr.Paused = false
		}
		j.Unlock()
	}
}

func (j *Jukebox) doUpdateSpeaker(su updateSpeaker) error {
	if su.index >= len(j.playlist) {
		j.Lock()
		j.playing = false
		j.Unlock()
		return nil
	}
	j.Lock()
	j.index = su.index
	j.Unlock()
	f, err := os.Open(path.Join(
		j.musicPath,
		j.playlist[su.index].RelPath(),
	))
	if err != nil {
		return err
	}
	var streamer beep.Streamer
	var format beep.Format
	switch j.playlist[su.index].Ext() {
	case "mp3":
		streamer, format, err = mp3.Decode(f)
	case "flac":
		streamer, format, err = flac.Decode(f)
	}
	if err != nil {
		return err
	}
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
		j.speaker <- updateSpeaker{su.index + 1}
	})))
	return nil
}

func (j *Jukebox) SetTracks(tracks []*db.Track) {
	j.updates <- update{
		action: set,
		tracks: tracks,
	}
}

func (j *Jukebox) AddTracks(tracks []*db.Track) {
	j.updates <- update{
		action: add,
		tracks: tracks,
	}
}

func (j *Jukebox) RemoveTrack(i int) {
	j.updates <- update{
		action: remove,
		index:  i,
	}
}

func (j *Jukebox) Skip(i int) {
	j.updates <- update{
		action: skip,
		index:  i,
	}
}

func (j *Jukebox) ClearTracks() {
	j.updates <- update{action: clear}
}

func (j *Jukebox) Stop() {
	j.updates <- update{action: stop}
}

func (j *Jukebox) Start() {
	j.updates <- update{action: start}
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
