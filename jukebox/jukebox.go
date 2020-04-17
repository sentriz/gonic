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

type strmInfo struct {
	ctrlStrmr beep.Ctrl
	strm      beep.StreamSeekCloser
	format    beep.Format
}

type Jukebox struct {
	playlist []*db.Track
	index    int
	playing  bool
	// used to notify the player to re read the members
	updates chan struct{}
	done    chan bool
	info    *strmInfo
	sync.Mutex
}

func (j *Jukebox) Init(musicPath string) error {
	j.updates = make(chan struct{})
	sr := beep.SampleRate(48000)
	err := speaker.Init(sr, sr.N(time.Second/2))
	if err != nil {
		return err
	}
	j.done = make(chan bool)
	go func() {
		for range j.updates {
			var streamer beep.Streamer
			var format beep.Format
			f, err := os.Open(path.Join(musicPath, j.playlist[j.index].RelPath()))
			if err != nil {
				j.index++
				continue
			}
			switch j.playlist[j.index].Ext() {
			case "mp3":
				streamer, format, err = mp3.Decode(f)
			case "flac":
				streamer, format, err = flac.Decode(f)
			default:
				j.index++
				continue
			}
			if err != nil {
				j.index++
				continue
			}
			if j.playing {
				j.Lock()
				{
					j.info = &strmInfo{}
					j.info.strm = streamer.(beep.StreamSeekCloser)
					j.info.ctrlStrmr.Streamer = beep.Resample(4, format.SampleRate, sr, j.info.strm)
					j.info.format = format
				}
				j.Unlock()
				speaker.Play(beep.Seq(&j.info.ctrlStrmr, beep.Callback(func() {
					j.done <- false
				})))
				if v := <-j.done; !v {
					j.index++
					j.Lock()
					if j.index > len(j.playlist) {
						j.index = 0
						j.playing = false
					}
					j.Unlock()
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
	return nil
}

func (j *Jukebox) SetTracks(tracks []*db.Track) {
	j.Lock()
	j.index = 0
	j.playing = true
	if len(j.playlist) > 0 {
		j.done <- true
		j.playlist = []*db.Track{}
		speaker.Clear()
	}
	j.playlist = tracks
	j.Unlock()
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
	j.playing = false
	j.info.ctrlStrmr.Paused = false
	j.Unlock()
}

func (j *Jukebox) Skip(i int, skipCurrent bool) {
	j.Lock()
	if skipCurrent {
		j.index++
	} else {
		j.index = i
	}
	speaker.Clear()
	j.done <- true
	j.updates <- struct{}{}
	j.Unlock()
}
