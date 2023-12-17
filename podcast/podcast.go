package podcast

import (
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jinzhu/gorm"
	"github.com/mmcdole/gofeed"

	"go.senan.xyz/gonic/db"
	"go.senan.xyz/gonic/fileutil"
	"go.senan.xyz/gonic/tags/tagcommon"
)

var ErrNoAudioInFeedItem = errors.New("no audio in feed item")

const (
	fetchUserAgent = `Mozilla/5.0 (Macintosh; Intel Mac OS X 10_7_5) AppleWebKit/537.11 (KHTML, like Gecko) Chrome/23.0.1271.64 Safari/537.11`
)

type Podcasts struct {
	db        *db.DB
	baseDir   string
	tagReader tagcommon.Reader
}

func New(db *db.DB, base string, tagReader tagcommon.Reader) *Podcasts {
	return &Podcasts{
		db:        db,
		baseDir:   base,
		tagReader: tagReader,
	}
}

func (p *Podcasts) GetPodcastOrAll(id int, includeEpisodes bool) ([]*db.Podcast, error) {
	var podcasts []*db.Podcast
	q := p.db.DB
	if id != 0 {
		q = q.Where("id=?", id)
	}
	if includeEpisodes {
		q = q.Preload("Episodes", func(db *gorm.DB) *gorm.DB {
			return db.Order("podcast_episodes.publish_date DESC")
		})
	}
	if err := q.Find(&podcasts).Error; err != nil {
		return nil, fmt.Errorf("find podcasts: %w", err)
	}
	return podcasts, nil
}

func (p *Podcasts) GetNewestPodcastEpisodes(count int) ([]*db.PodcastEpisode, error) {
	var episodes []*db.PodcastEpisode
	err := p.db.
		Order("publish_date DESC").
		Limit(count).
		Find(&episodes).
		Error
	if err != nil {
		return nil, fmt.Errorf("find newest podcast episodes: %w", err)
	}
	return episodes, nil
}

func (p *Podcasts) AddNewPodcast(rssURL string, feed *gofeed.Feed) (*db.Podcast, error) {
	rootDir, err := fileutil.Unique(filepath.Join(p.baseDir, fileutil.Safe(feed.Title)), "")
	if err != nil {
		return nil, fmt.Errorf("find unique podcast dir: %w", err)
	}
	podcast := db.Podcast{
		Description: feed.Description,
		ImageURL:    feed.Image.URL,
		Title:       feed.Title,
		URL:         rssURL,
		RootDir:     rootDir,
	}
	if err := os.Mkdir(podcast.RootDir, 0o755); err != nil && !os.IsExist(err) {
		return nil, err
	}
	if err := p.db.Save(&podcast).Error; err != nil {
		return &podcast, err
	}
	if err := p.RefreshPodcast(&podcast, feed.Items); err != nil {
		log.Printf("error addign new episodes : %v", err)
	}
	if err := p.downloadPodcastCover(&podcast); err != nil {
		log.Printf("error downloading podcast cover: %v", err)
	}
	return &podcast, nil
}

func (p *Podcasts) SetAutoDownload(podcastID int, setting db.PodcastAutoDownload) error {
	podcast := db.Podcast{}
	err := p.db.
		Where("id=?", podcastID).
		First(&podcast).
		Error
	if err != nil {
		return err
	}
	podcast.AutoDownload = setting
	if err := p.db.Save(&podcast).Error; err != nil {
		return fmt.Errorf("save setting: %w", err)
	}
	return nil
}

func getEntriesAfterDate(feed []*gofeed.Item, after time.Time) []*gofeed.Item {
	items := []*gofeed.Item{}
	for _, item := range feed {
		if item.PublishedParsed.Before(after) || item.PublishedParsed.Equal(after) {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (p *Podcasts) RefreshPodcast(podcast *db.Podcast, items []*gofeed.Item) error {
	var lastPodcastEpisode db.PodcastEpisode
	err := p.db.
		Where("podcast_id=?", podcast.ID).
		Order("publish_date DESC").
		First(&lastPodcastEpisode).
		Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	if lastPodcastEpisode.ID != 0 {
		items = getEntriesAfterDate(items, *lastPodcastEpisode.PublishDate)
	}

	var episodeErrs []error
	for _, item := range items {
		podcastEpisode, err := p.addEpisode(podcast.ID, item)
		if err != nil {
			episodeErrs = append(episodeErrs, err)
			continue
		}

		if lastPodcastEpisode.ID != 0 && podcast.AutoDownload == db.PodcastAutoDownloadLatest {
			podcastEpisode.Status = db.PodcastEpisodeStatusDownloading
			if err := p.db.Save(&podcastEpisode).Error; err != nil {
				return fmt.Errorf("save podcast episode: %w", err)
			}
		}
	}

	return errors.Join(episodeErrs...)
}

func (p *Podcasts) addEpisode(podcastID int, item *gofeed.Item) (*db.PodcastEpisode, error) {
	var duration int
	// if it has the media extension use it
	for _, content := range item.Extensions["media"]["content"] {
		durationExt := content.Attrs["duration"]
		duration = getSecondsFromString(durationExt)
		if duration != 0 {
			break
		}
	}
	// if the itunes extension is available, use AddEpisode
	if duration == 0 && item.ITunesExt != nil {
		duration = getSecondsFromString(item.ITunesExt.Duration)
	}

	if episode, ok := p.findEnclosureAudio(podcastID, duration, item); ok {
		if err := p.db.Save(episode).Error; err != nil {
			return nil, err
		}
		return episode, nil
	}
	if episode, ok := p.findMediaAudio(podcastID, duration, item); ok {
		if err := p.db.Save(episode).Error; err != nil {
			return nil, err
		}
		return episode, nil
	}
	return nil, ErrNoAudioInFeedItem
}

func (p *Podcasts) isAudio(rawItemURL string) (bool, error) {
	itemURL, err := url.Parse(rawItemURL)
	if err != nil {
		return false, err
	}
	return p.tagReader.CanRead(itemURL.Path), nil
}

func itemToEpisode(podcastID, size, duration int, audio string, item *gofeed.Item) *db.PodcastEpisode {
	return &db.PodcastEpisode{
		PodcastID:   podcastID,
		Description: item.Description,
		Title:       item.Title,
		Length:      duration,
		Size:        size,
		PublishDate: item.PublishedParsed,
		AudioURL:    audio,
		Status:      db.PodcastEpisodeStatusSkipped,
	}
}

func (p *Podcasts) findEnclosureAudio(podcastID, duration int, item *gofeed.Item) (*db.PodcastEpisode, bool) {
	for _, enc := range item.Enclosures {
		if t, err := p.isAudio(enc.URL); !t || err != nil {
			continue
		}
		size, _ := strconv.Atoi(enc.Length)
		return itemToEpisode(podcastID, size, duration, enc.URL, item), true
	}
	return nil, false
}

func (p *Podcasts) findMediaAudio(podcastID, duration int, item *gofeed.Item) (*db.PodcastEpisode, bool) {
	extensions, ok := item.Extensions["media"]["content"]
	if !ok {
		return nil, false
	}
	for _, ext := range extensions {
		if t, err := p.isAudio(ext.Attrs["url"]); !t || err != nil {
			continue
		}
		return itemToEpisode(podcastID, 0, duration, ext.Attrs["url"], item), true
	}
	return nil, false
}

func (p *Podcasts) RefreshPodcasts() error {
	var podcasts []*db.Podcast
	if err := p.db.Find(&podcasts).Error; err != nil {
		return fmt.Errorf("find podcasts: %w", err)
	}

	var errs []error
	for _, podcast := range podcasts {
		fp := gofeed.NewParser()
		feed, err := fp.ParseURL(podcast.URL)
		if err != nil {
			errs = append(errs, fmt.Errorf("refreshing podcast with url %q: %w", podcast.URL, err))
			continue
		}
		if err = p.RefreshPodcast(podcast, feed.Items); err != nil {
			errs = append(errs, fmt.Errorf("adding episodes: %w", err))
			continue
		}
	}
	return errors.Join(errs...)
}

func (p *Podcasts) DownloadPodcastAll(podcastID int) error {
	err := p.db.
		Model(db.PodcastEpisode{}).
		Where("status=?", db.PodcastEpisodeStatusSkipped).
		Where("podcast_id=?", podcastID).
		Update("status", db.PodcastEpisodeStatusDownloading).
		Error
	if err != nil {
		return fmt.Errorf("update podcast episodes: %w", err)
	}
	return nil
}

func (p *Podcasts) DownloadEpisode(episodeID int) error {
	err := p.db.
		Model(db.PodcastEpisode{}).
		Where("id=?", episodeID).
		Update("status", db.PodcastEpisodeStatusDownloading).
		Error
	if err != nil {
		return fmt.Errorf("update podcast episodes: %w", err)
	}
	return nil
}

func getContentDispositionFilename(header http.Header) (string, bool) {
	contentHeader := header.Get("content-disposition")
	_, params, _ := mime.ParseMediaType(contentHeader)
	filename, ok := params["filename"]
	return filename, ok
}

func getPodcastEpisodeFilename(podcast *db.Podcast, podcastEpisode *db.PodcastEpisode, header http.Header) (string, error) {
	if podcastEpisode.Filename != "" {
		return podcastEpisode.Filename, nil
	}

	filename, ok := getContentDispositionFilename(header)
	if !ok {
		audioURL, err := url.Parse(podcastEpisode.AudioURL)
		if err != nil {
			return "", fmt.Errorf("parse podcast audio url: %w", err)
		}
		filename = path.Base(audioURL.Path)
	}
	path, err := fileutil.Unique(podcast.RootDir, fileutil.Safe(filename))
	if err != nil {
		return "", fmt.Errorf("find unique path: %w", err)
	}
	_, filename = filepath.Split(path)

	return filename, nil
}

func (p *Podcasts) downloadPodcastCover(podcast *db.Podcast) error {
	imageURL, err := url.Parse(podcast.ImageURL)
	if err != nil {
		return fmt.Errorf("parse image url: %w", err)
	}
	req, err := http.NewRequest("GET", podcast.ImageURL, nil)
	if err != nil {
		return fmt.Errorf("create http request: %w", err)
	}
	req.Header.Add("User-Agent", fetchUserAgent)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch image url: %w", err)
	}
	defer resp.Body.Close()

	var ext = path.Ext(imageURL.Path)
	if ext == "" {
		filename, _ := getContentDispositionFilename(resp.Header)
		ext = filepath.Ext(filename)
	}

	if err := os.MkdirAll(podcast.RootDir, os.ModePerm); err != nil {
		return fmt.Errorf("make podcast root dir: %w", err)
	}
	coverFile, err := os.Create(filepath.Join(podcast.RootDir, "cover"+ext))
	if err != nil {
		return fmt.Errorf("creating podcast cover: %w", err)
	}
	defer coverFile.Close()

	if _, err := io.Copy(coverFile, resp.Body); err != nil {
		return fmt.Errorf("writing podcast cover: %w", err)
	}
	podcast.Image = fmt.Sprintf("cover%s", ext)
	if err := p.db.Save(podcast).Error; err != nil {
		return fmt.Errorf("save podcast: %w", err)
	}
	return nil
}

func (p *Podcasts) DeletePodcast(podcastID int) error {
	var podcast db.Podcast
	if err := p.db.Where("id=?", podcastID).First(&podcast).Error; err != nil {
		return err
	}
	if podcast.RootDir == "" {
		return fmt.Errorf("podcast has no root dir")
	}

	if err := os.RemoveAll(podcast.RootDir); err != nil {
		return fmt.Errorf("delete podcast directory: %w", err)
	}

	if err := p.db.Where("id=?", podcastID).Delete(db.Podcast{}).Error; err != nil {
		return fmt.Errorf("delete podcast row: %w", err)
	}
	return nil
}

func (p *Podcasts) DeletePodcastEpisode(podcastEpisodeID int) error {
	var podcastEpisode db.PodcastEpisode
	if err := p.db.Preload("Podcast").First(&podcastEpisode, podcastEpisodeID).Error; err != nil {
		return err
	}
	podcastEpisode.Status = db.PodcastEpisodeStatusDeleted
	if err := p.db.Save(&podcastEpisode).Error; err != nil {
		return fmt.Errorf("save podcast episode: %w", err)
	}
	if err := os.Remove(podcastEpisode.AbsPath()); err != nil {
		return fmt.Errorf("remove episode: %w", err)
	}
	return nil
}

func (p *Podcasts) PurgeOldPodcasts(maxAge time.Duration) error {
	expDate := time.Now().Add(-maxAge)
	var episodes []*db.PodcastEpisode
	err := p.db.
		Where("status=?", db.PodcastEpisodeStatusCompleted).
		Where("created_at<?", expDate).
		Where("updated_at<?", expDate).
		Where("modified_at<?", expDate).
		Preload("Podcast").
		Find(&episodes).
		Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("find podcasts: %w", err)
	}
	for _, episode := range episodes {
		episode.Status = db.PodcastEpisodeStatusDeleted
		if err := p.db.Save(episode).Error; err != nil {
			return fmt.Errorf("save new podcast status: %w", err)
		}
		if episode.Podcast == nil {
			return fmt.Errorf("episode %d has no podcast", episode.ID)
		}
		if err := os.Remove(episode.AbsPath()); err != nil {
			return fmt.Errorf("remove podcast path: %w", err)
		}
	}
	return nil
}

func getSecondsFromString(time string) int {
	duration, err := strconv.Atoi(time)
	if err == nil {
		return duration
	}
	splitTime := strings.Split(time, ":")
	if len(splitTime) == 3 {
		hours, _ := strconv.Atoi(splitTime[0])
		minutes, _ := strconv.Atoi(splitTime[1])
		seconds, _ := strconv.Atoi(splitTime[2])
		return (3600 * hours) + (60 * minutes) + seconds
	}
	if len(splitTime) == 2 {
		minutes, _ := strconv.Atoi(splitTime[0])
		seconds, _ := strconv.Atoi(splitTime[1])
		return (60 * minutes) + seconds
	}
	return 0
}

func (p *Podcasts) DownloadTick() error {
	var podcastEpisode db.PodcastEpisode
	err := p.db.
		Preload("Podcast").
		Where("status=?", db.PodcastEpisodeStatusDownloading).
		Order("updated_at DESC").
		Find(&podcastEpisode).
		Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("find episode: %w", err)
	}
	if podcastEpisode.ID == 0 {
		return nil
	}

	if err := p.doPodcastDownload(podcastEpisode.Podcast, &podcastEpisode); err != nil {
		return fmt.Errorf("do download: %w", err)
	}
	return nil
}

func (p *Podcasts) doPodcastDownload(podcast *db.Podcast, podcastEpisode *db.PodcastEpisode) (err error) {
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodGet, podcastEpisode.AudioURL, nil)
	if err != nil {
		return fmt.Errorf("create http request: %w", err)
	}
	req.Header.Add("User-Agent", fetchUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch podcast audio: %w", err)
	}
	defer resp.Body.Close()

	filename, err := getPodcastEpisodeFilename(podcast, podcastEpisode, resp.Header)
	if err != nil {
		return fmt.Errorf("get podcast episode filename: %w", err)
	}
	podcastEpisode.Filename = filename
	if err := p.db.Save(&podcastEpisode).Error; err != nil {
		return fmt.Errorf("save podcast episode: %w", err)
	}
	file, err := os.Create(filepath.Join(podcast.RootDir, podcastEpisode.Filename))
	if err != nil {
		return fmt.Errorf("create audio file: %w", err)
	}
	defer file.Close()

	defer func() {
		if err != nil {
			podcastEpisode.Status = db.PodcastEpisodeStatusError
			_ = p.db.Save(&podcastEpisode).Error
		}
	}()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return fmt.Errorf("writing podcast episode: %w", err)
	}

	podcastTags, err := p.tagReader.Read(podcastEpisode.AbsPath())
	if err != nil {
		return fmt.Errorf("read podcast tags: %w", err)
	}

	podcastEpisode.Status = db.PodcastEpisodeStatusCompleted
	podcastEpisode.Bitrate = podcastTags.Bitrate()
	podcastEpisode.Length = podcastTags.Length()

	stat, _ := file.Stat()
	podcastEpisode.Size = int(stat.Size())

	if err := p.db.Save(podcastEpisode).Error; err != nil {
		return fmt.Errorf("save podcast episode: %w", err)
	}
	return nil
}
