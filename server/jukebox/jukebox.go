// author: AlexKraak (https://github.com/alexkraak/)

package jukebox

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"

	"go.senan.xyz/gonic/server/db"
)

type Status struct {
	CurrentIndex int
	Playing      bool
	Gain         float64
	Position     int
}

type Jukebox struct {
	playlist []*db.Track
	index    int
	playing  bool
	sr       beep.SampleRate
	// used to notify the player to re read the members
	quit    chan struct{}
	done    chan bool
	info    *strmInfo
	speaker chan updateSpeaker
	sync.Mutex
}

type strmInfo struct {
	ctrlStrmr beep.Ctrl
	strm      beep.StreamSeekCloser
	format    beep.Format
}

type updateSpeaker struct {
	index  int
	offset int
}

func New() *Jukebox {
	return &Jukebox{
		sr:      beep.SampleRate(48000),
		speaker: make(chan updateSpeaker, 1),
		done:    make(chan bool),
		quit:    make(chan struct{}),
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
		case speaker := <-j.speaker:
			if err := j.doUpdateSpeaker(speaker); err != nil {
				log.Printf("error in jukebox: %v", err)
			}
		}
	}
}

func (j *Jukebox) Quit() {
	j.quit <- struct{}{}
}

func (j *Jukebox) doUpdateSpeaker(su updateSpeaker) error {
	j.Lock()
	defer j.Unlock()
	if su.index >= len(j.playlist) {
		j.playing = false
		speaker.Clear()
		return nil
	}
	j.index = su.index
	f, err := os.Open(j.playlist[su.index].AbsPath())
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
	j.info = &strmInfo{}
	j.info.strm = streamer.(beep.StreamSeekCloser)
	if su.offset != 0 {
		samples := format.SampleRate.N(time.Second * time.Duration(su.offset))
		if err := j.info.strm.Seek(samples); err != nil {
			return err
		}
	}
	j.info.ctrlStrmr.Streamer = beep.Resample(
		4, format.SampleRate,
		j.sr, j.info.strm,
	)
	j.info.format = format
	speaker.Play(beep.Seq(&j.info.ctrlStrmr, beep.Callback(func() {
		j.speaker <- updateSpeaker{index: su.index + 1}
	})))
	return nil
}

func (j *Jukebox) SetTracks(tracks []*db.Track) {
	j.Lock()
	defer j.Unlock()
	j.playlist = tracks
}

func (j *Jukebox) AddTracks(tracks []*db.Track) {
	j.Lock()
	if len(j.playlist) == 0 {
		j.playlist = tracks
		j.playing = true
		j.index = 0
		j.Unlock()
		j.speaker <- updateSpeaker{index: 0}
		return
	}
	j.playlist = append(j.playlist, tracks...)
	j.Unlock()
}

func (j *Jukebox) RemoveTrack(i int) {
	j.Lock()
	defer j.Unlock()
	if i < 0 || i >= len(j.playlist) {
		return
	}
	j.playlist = append(j.playlist[:i], j.playlist[i+1:]...)
}

func (j *Jukebox) Skip(i int, offset int) {
	speaker.Clear()
	j.Lock()
	j.index = i
	j.playing = true
	j.Unlock()
	j.speaker <- updateSpeaker{index: j.index, offset: offset}
}

func (j *Jukebox) ClearTracks() {
	speaker.Clear()
	j.Lock()
	defer j.Unlock()
	j.playing = false
	j.playlist = []*db.Track{}
}

func (j *Jukebox) Stop() {
	j.Lock()
	defer j.Unlock()
	if j.info != nil {
		j.playing = false
		j.info.ctrlStrmr.Paused = true
	}
}

func (j *Jukebox) Start() {
	if j.info != nil {
		j.playing = true
		j.info.ctrlStrmr.Paused = false
	}
}

func (j *Jukebox) GetStatus() Status {
	j.Lock()
	defer j.Unlock()
	position := 0
	if j.info != nil {
		length := j.info.format.SampleRate.D(j.info.strm.Position())
		position = int(length.Round(time.Millisecond).Seconds())
	}
	return Status{
		CurrentIndex: j.index,
		Playing:      j.playing,
		Gain:         0.9,
		Position:     position,
	}
}

func (j *Jukebox) GetTracks() []*db.Track {
	j.Lock()
	defer j.Unlock()
	return j.playlist
}
