// author: AlexKraak (https://github.com/alexkraak/)
// author: sentriz (https://github.com/sentriz/)

package jukebox

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/dexterlb/mpvipc"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/exp/slices"
)

var (
	ErrMPVTimeout      = fmt.Errorf("mpv not responding")
	ErrMPVNeverStarted = fmt.Errorf("mpv never started")
	ErrMPVTooOld       = fmt.Errorf("mpv too old")
)

func MPVArg(k string, v any) string {
	if v, ok := v.(bool); ok {
		if v {
			return fmt.Sprintf("%s=yes", k)
		}
		return fmt.Sprintf("%s=no", k)
	}
	return fmt.Sprintf("%s=%v", k, v)
}

type Jukebox struct {
	cmd    *exec.Cmd
	conn   *mpvipc.Connection
	events <-chan *mpvipc.Event

	mu sync.Mutex
}

func New() *Jukebox {
	return &Jukebox{}
}

func (j *Jukebox) Start(sockPath string, mpvExtraArgs []string) error {
	const mpvName = "mpv"
	if _, err := exec.LookPath(mpvName); err != nil {
		return fmt.Errorf("look path: %w. did you forget to install it?", err)
	}

	var mpvArgs []string
	mpvArgs = append(mpvArgs, "--idle", "--no-config", "--no-video", MPVArg("--audio-display", "no"), MPVArg("--input-ipc-server", sockPath))
	mpvArgs = append(mpvArgs, mpvExtraArgs...)

	j.cmd = exec.Command(mpvName, mpvArgs...)
	if err := j.cmd.Start(); err != nil {
		return fmt.Errorf("start mpv process: %w", err)
	}

	ok := waitUntil(5*time.Second, func() bool {
		_, err := os.Stat(sockPath)
		return err == nil
	})
	if !ok {
		_ = j.cmd.Process.Kill()
		return ErrMPVNeverStarted
	}

	j.conn = mpvipc.NewConnection(sockPath)
	if err := j.conn.Open(); err != nil {
		return fmt.Errorf("open connection: %w", err)
	}

	var mpvVersionStr string
	if err := j.getDecode(&mpvVersionStr, "mpv-version"); err != nil {
		return fmt.Errorf("get mpv version: %w", err)
	}
	if major, minor, patch := parseMPVVersion(mpvVersionStr); major == 0 && minor < 34 {
		return fmt.Errorf("%w: v0.34.0+ required, found v%d.%d.%d", ErrMPVTooOld, major, minor, patch)
	}

	if _, err := j.conn.Call("observe_property", 0, "seekable"); err != nil {
		return fmt.Errorf("observe property: %w", err)
	}
	j.events, _ = j.conn.NewEventListener()

	return nil
}

func (j *Jukebox) Wait() error {
	var exitError *exec.ExitError
	if err := j.cmd.Wait(); err != nil && !errors.As(err, &exitError) {
		return fmt.Errorf("wait jukebox: %w", err)
	}
	return nil
}

func (j *Jukebox) GetPlaylist() ([]string, error) {
	defer lock(&j.mu)()

	var playlist mpvPlaylist
	if err := j.getDecode(&playlist, "playlist"); err != nil {
		return nil, fmt.Errorf("get playlist: %w", err)
	}
	var items []string
	for _, item := range playlist {
		items = append(items, item.Filename)
	}
	return items, nil
}

func (j *Jukebox) SetPlaylist(items []string) error {
	defer lock(&j.mu)()

	var playlist mpvPlaylist
	if err := j.getDecode(&playlist, "playlist"); err != nil {
		return fmt.Errorf("get playlist: %w", err)
	}
	current, currentIndex := find(playlist, func(item mpvPlaylistItem) bool {
		return item.Current
	})

	filteredItems, foundExistingTrack := filter(items, func(filename string) bool {
		return filename != current.Filename
	})

	tmp, cleanup, err := tmp()
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer cleanup()
	for _, item := range filteredItems {
		item, _ = filepath.Abs(item)
		fmt.Fprintln(tmp, item)
	}

	if !foundExistingTrack {
		// easy case - a brand new set of tracks that we can overwrite
		if _, err := j.conn.Call("loadlist", tmp.Name(), "replace"); err != nil {
			return fmt.Errorf("load list: %w", err)
		}
		return nil
	}

	// not so easy, we need to clear the playlist except what's playing, load everything
	// except for what we're playing, then move what's playing back to its original index
	// clear all items except what's playing (will be at index 0)
	if _, err := j.conn.Call("playlist-clear"); err != nil {
		return fmt.Errorf("clear playlist: %w", err)
	}
	if _, err := j.conn.Call("loadlist", tmp.Name(), "append-play"); err != nil {
		return fmt.Errorf("load list: %w", err)
	}
	if _, err := j.conn.Call("playlist-move", 0, currentIndex+1); err != nil {
		return fmt.Errorf("playlist move: %w", err)
	}
	return nil
}

func (j *Jukebox) AppendToPlaylist(items []string) error {
	defer lock(&j.mu)()

	tmp, cleanup, err := tmp()
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer cleanup()
	for _, item := range items {
		fmt.Fprintln(tmp, item)
	}
	if _, err := j.conn.Call("loadlist", tmp.Name(), "append"); err != nil {
		return fmt.Errorf("load list: %w", err)
	}
	return nil
}

func (j *Jukebox) RemovePlaylistIndex(i int) error {
	defer lock(&j.mu)()

	if _, err := j.conn.Call("playlist-remove", i); err != nil {
		return fmt.Errorf("playlist remove: %w", err)
	}
	return nil
}

func (j *Jukebox) SkipToPlaylistIndex(i int, offsetSecs int) error {
	defer lock(&j.mu)()

	matchEventSeekable := func(e *mpvipc.Event) bool {
		seekable, _ := e.Data.(bool)
		return e.Name == "property-change" &&
			e.ExtraData["name"] == "seekable" &&
			seekable
	}

	if offsetSecs > 0 {
		if err := j.conn.Set("pause", true); err != nil {
			return fmt.Errorf("pause: %w", err)
		}
	}
	if _, err := j.conn.Call("playlist-play-index", i); err != nil {
		return fmt.Errorf("playlist play index: %w", err)
	}
	if offsetSecs > 0 {
		if err := waitFor(j.events, matchEventSeekable); err != nil {
			return fmt.Errorf("waiting for file load: %w", err)
		}
		if _, err := j.conn.Call("seek", offsetSecs, "absolute"); err != nil {
			return fmt.Errorf("seek: %w", err)
		}
		if err := j.conn.Set("pause", false); err != nil {
			return fmt.Errorf("play: %w", err)
		}
	}
	return nil
}

func (j *Jukebox) ClearPlaylist() error {
	defer lock(&j.mu)()

	if _, err := j.conn.Call("playlist-clear"); err != nil {
		return fmt.Errorf("seek: %w", err)
	}
	return nil
}

func (j *Jukebox) Pause() error {
	defer lock(&j.mu)()

	if err := j.conn.Set("pause", true); err != nil {
		return fmt.Errorf("pause: %w", err)
	}
	return nil
}

func (j *Jukebox) Play() error {
	defer lock(&j.mu)()

	if err := j.conn.Set("pause", false); err != nil {
		return fmt.Errorf("pause: %w", err)
	}
	return nil
}

func (j *Jukebox) SetVolumePct(v int) error {
	defer lock(&j.mu)()

	if err := j.conn.Set("volume", v); err != nil {
		return fmt.Errorf("set volume: %w", err)
	}
	return nil
}

func (j *Jukebox) GetVolumePct() (float64, error) {
	defer lock(&j.mu)()

	var volume float64
	if err := j.getDecode(&volume, "volume"); err != nil {
		return 0, fmt.Errorf("get volume: %w", err)
	}
	return volume, nil
}

type Status struct {
	CurrentIndex    int
	CurrentFilename string
	Length          int
	Playing         bool
	GainPct         int
	Position        int
}

func (j *Jukebox) GetStatus() (*Status, error) {
	defer lock(&j.mu)()

	var status Status
	_ = j.getDecode(&status.Position, "time-pos") // property may not always be there
	_ = j.getDecode(&status.GainPct, "volume")    // property may not always be there

	var playlist mpvPlaylist
	_ = j.getDecode(&playlist, "playlist")

	status.CurrentIndex = slices.IndexFunc(playlist, func(pl mpvPlaylistItem) bool {
		return pl.Current
	})

	status.Length = len(playlist)

	if status.CurrentIndex < 0 {
		return &status, nil
	}

	status.CurrentFilename = playlist[status.CurrentIndex].Filename

	var paused bool
	_ = j.getDecode(&paused, "pause") // property may not always be there
	status.Playing = !paused

	return &status, nil
}

func (j *Jukebox) Quit() error {
	defer lock(&j.mu)()

	if j.conn == nil || j.conn.IsClosed() {
		return nil
	}
	go func() {
		_, _ = j.conn.Call("quit")
	}()

	time.Sleep(250 * time.Millisecond)
	_ = j.cmd.Process.Kill()

	if err := j.conn.Close(); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	j.conn.WaitUntilClosed()
	return nil
}

func (j *Jukebox) getDecode(dest any, property string) error {
	raw, err := j.conn.Get(property)
	if err != nil {
		return fmt.Errorf("get property: %w", err)
	}
	if err := mapstructure.Decode(raw, dest); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	return nil
}

type (
	mpvPlaylist     []mpvPlaylistItem
	mpvPlaylistItem struct {
		ID       int
		Filename string
		Current  bool
		Playing  bool
	}
)

func waitUntil(timeout time.Duration, f func() bool) bool {
	quit := time.NewTicker(timeout)
	defer quit.Stop()
	check := time.NewTicker(100 * time.Millisecond)
	defer check.Stop()

	for {
		select {
		case <-quit.C:
			return false
		case <-check.C:
			if f() {
				return true
			}
		}
	}
}

func waitFor[T any](ch <-chan T, match func(e T) bool) error {
	quit := time.NewTicker(1 * time.Second)
	defer quit.Stop()

	defer time.Sleep(350 * time.Millisecond)

	for {
		select {
		case <-quit.C:
			return ErrMPVTimeout
		case ev := <-ch:
			if match(ev) {
				return nil
			}
		}
	}
}

func tmp() (*os.File, func(), error) {
	tmp, err := os.CreateTemp("", "")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp file: %w", err)
	}
	cleanup := func() {
		os.Remove(tmp.Name())
		tmp.Close()
	}
	return tmp, cleanup, nil
}

func find[T any](items []T, f func(T) bool) (T, int) {
	for i, item := range items {
		if f(item) {
			return item, i
		}
	}
	var t T
	return t, -1
}

func filter[T comparable](items []T, f func(T) bool) ([]T, bool) {
	var found bool
	var ret []T
	for _, item := range items {
		if !f(item) {
			found = true
			continue
		}
		ret = append(ret, item)
	}
	return ret, found
}

func lock(mu *sync.Mutex) func() {
	mu.Lock()
	return mu.Unlock
}

var mpvVersionExpr = regexp.MustCompile(`mpv\s(\d+)\.(\d+)\.(\d+)`)

func parseMPVVersion(version string) (major, minor, patch int) {
	m := mpvVersionExpr.FindStringSubmatch(version)
	if len(m) != 4 {
		return
	}
	major, _ = strconv.Atoi(m[1])
	minor, _ = strconv.Atoi(m[2])
	patch, _ = strconv.Atoi(m[3])
	return
}
