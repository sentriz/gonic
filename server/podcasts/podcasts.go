package podcasts

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

	"go.senan.xyz/gonic/multierr"
	"go.senan.xyz/gonic/server/db"
	gmime "go.senan.xyz/gonic/server/mime"
	"go.senan.xyz/gonic/server/scanner/tags"
)

const downloadAllWaitInterval = 3 * time.Second

type Podcasts struct {
	db      *db.DB
	baseDir string
	tagger  tags.Reader
}

func New(db *db.DB, base string, tagger tags.Reader) *Podcasts {
	return &Podcasts{
		db:      db,
		baseDir: base,
		tagger:  tagger,
	}
}

func (p *Podcasts) GetPodcastOrAll(userID int, id int, includeEpisodes bool) ([]*db.Podcast, error) {
	podcasts := []*db.Podcast{}
	q := p.db.Where("user_id=?", userID)
	if id != 0 {
		q = q.Where("id=?", id)
	}
	if err := q.Find(&podcasts).Error; err != nil {
		return nil, fmt.Errorf("finding podcasts: %w", err)
	}
	if !includeEpisodes {
		return podcasts, nil
	}
	for _, c := range podcasts {
		episodes, err := p.GetPodcastEpisodes(c.ID)
		if err != nil {
			return nil, fmt.Errorf("finding podcast episodes: %w", err)
		}
		c.Episodes = episodes
	}
	return podcasts, nil
}

func (p *Podcasts) GetPodcastEpisodes(podcastID int) ([]*db.PodcastEpisode, error) {
	episodes := []*db.PodcastEpisode{}
	err := p.db.
		Where("podcast_id=?", podcastID).
		Order("publish_date DESC").
		Find(&episodes).
		Error
	if err != nil {
		return nil, fmt.Errorf("find episodes by podcast id: %w", err)
	}
	return episodes, nil
}

func (p *Podcasts) AddNewPodcast(rssURL string, feed *gofeed.Feed,
	userID int) (*db.Podcast, error) {
	podcast := db.Podcast{
		Description: feed.Description,
		ImageURL:    feed.Image.URL,
		UserID:      userID,
		Title:       feed.Title,
		URL:         rssURL,
	}
	podPath := podcast.Fullpath(p.baseDir)
	err := os.Mkdir(podPath, 0755)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	if err := p.db.Save(&podcast).Error; err != nil {
		return &podcast, err
	}
	if err := p.AddNewEpisodes(&podcast, feed.Items); err != nil {
		return nil, err
	}
	go func() {
		if err := p.downloadPodcastCover(podPath, &podcast); err != nil {
			log.Printf("error downloading podcast cover: %v", err)
		}
	}()
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

func (p *Podcasts) AddNewEpisodes(podcast *db.Podcast, items []*gofeed.Item) error {
	podcastEpisode := db.PodcastEpisode{}
	err := p.db.
		Where("podcast_id=?", podcast.ID).
		Order("publish_date DESC").
		First(&podcastEpisode).Error
	itemFound := true
	if errors.Is(err, gorm.ErrRecordNotFound) {
		itemFound = false
	} else if err != nil {
		return err
	}
	if !itemFound {
		for _, item := range items {
			if _, err := p.AddEpisode(podcast.ID, item); err != nil {
				return err
			}
		}
		return nil
	}
	for _, item := range getEntriesAfterDate(items, *podcastEpisode.PublishDate) {
		episode, err := p.AddEpisode(podcast.ID, item)
		if err != nil {
			return err
		}
		if podcast.AutoDownload == db.PodcastAutoDownloadLatest &&
			(episode.Status != db.PodcastEpisodeStatusCompleted && episode.Status != db.PodcastEpisodeStatusDownloading) {
			if err := p.DownloadEpisode(episode.ID); err != nil {
				return err
			}
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

func (p *Podcasts) AddEpisode(podcastID int, item *gofeed.Item) (*db.PodcastEpisode, error) {
	duration := 0
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
	// hopefully shouldnt reach here
	log.Println("failed to find audio in feed item, skipping")
	return nil, nil
}

func isAudio(mediaType, url string) bool {
	if mediaType != "" && strings.HasPrefix(mediaType, "audio") {
		return true
	}
	_, ok := gmime.FromExtension(filepath.Ext(url)[1:])
	return ok
}

func itemToEpisode(podcastID, size, duration int, audio string,
	item *gofeed.Item) *db.PodcastEpisode {
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

func (p *Podcasts) findEnclosureAudio(podcastID, duration int,
	item *gofeed.Item) (*db.PodcastEpisode, bool) {
	for _, enc := range item.Enclosures {
		if !isAudio(enc.Type, enc.URL) {
			continue
		}
		size, _ := strconv.Atoi(enc.Length)
		return itemToEpisode(podcastID, size, duration, enc.URL, item), true
	}
	return nil, false
}

func (p *Podcasts) findMediaAudio(podcastID, duration int,
	item *gofeed.Item) (*db.PodcastEpisode, bool) {
	extensions, ok := item.Extensions["media"]["content"]
	if !ok {
		return nil, false
	}
	for _, ext := range extensions {
		if !isAudio(ext.Attrs["type"], ext.Attrs["url"]) {
			continue
		}
		return itemToEpisode(podcastID, 0, duration, ext.Attrs["url"], item), true
	}
	return nil, false
}

func (p *Podcasts) RefreshPodcasts() error {
	podcasts := []*db.Podcast{}
	if err := p.db.Find(&podcasts).Error; err != nil {
		return fmt.Errorf("find podcasts: %w", err)
	}
	var errs *multierr.Err
	if errors.As(p.refreshPodcasts(podcasts), &errs) && errs.Len() > 0 {
		return fmt.Errorf("refresh podcasts: %w", errs)
	}
	return nil
}

func (p *Podcasts) RefreshPodcastsForUser(userID int) error {
	podcasts := []*db.Podcast{}
	err := p.db.
		Where("user_id=?", userID).
		Find(&podcasts).
		Error
	if err != nil {
		return fmt.Errorf("find podcasts: %w", err)
	}
	var errs *multierr.Err
	if errors.As(p.refreshPodcasts(podcasts), &errs) && errs.Len() > 0 {
		return fmt.Errorf("refresh podcasts: %w", errs)
	}
	return nil
}

func (p *Podcasts) refreshPodcasts(podcasts []*db.Podcast) error {
	errs := &multierr.Err{}
	for _, podcast := range podcasts {
		fp := gofeed.NewParser()
		feed, err := fp.ParseURL(podcast.URL)
		if err != nil {
			errs.Add(fmt.Errorf("refreshing podcast with url %q: %w", podcast.URL, err))
			continue
		}
		if err = p.AddNewEpisodes(podcast, feed.Items); err != nil {
			errs.Add(fmt.Errorf("adding episodes: %w", err))
			continue
		}
	}
	return errs
}

func (p *Podcasts) DownloadPodcastAll(podcastID int) error {
	podcastEpisodes := []db.PodcastEpisode{}
	err := p.db.
		Where("podcast_id=?", podcastID).
		Find(&podcastEpisodes).
		Error
	if err != nil {
		return fmt.Errorf("get episodes by podcast id: %w", err)
	}
	go func() {
		for _, episode := range podcastEpisodes {
			if episode.Status == db.PodcastEpisodeStatusDownloading || episode.Status == db.PodcastEpisodeStatusCompleted {
				log.Println("skipping episode is in progress or already downloaded")
				continue
			}
			if err := p.DownloadEpisode(episode.ID); err != nil {
				log.Printf("error downloading episode: %v", err)
				continue
			}
			log.Printf("finished downloading episode: %q", episode.Title)
			time.Sleep(downloadAllWaitInterval)
		}
	}()
	return nil
}

func (p *Podcasts) DownloadEpisode(episodeID int) error {
	podcastEpisode := db.PodcastEpisode{}
	podcast := db.Podcast{}
	err := p.db.
		Where("id=?", episodeID).
		First(&podcastEpisode).
		Error
	if err != nil {
		return fmt.Errorf("get podcast episode by id: %w", err)
	}
	err = p.db.
		Where("id=?", podcastEpisode.PodcastID).
		First(&podcast).
		Error
	if err != nil {
		return fmt.Errorf("get podcast by id: %w", err)
	}
	if podcastEpisode.Status == db.PodcastEpisodeStatusDownloading {
		log.Printf("Already downloading podcast episode with id %d", episodeID)
		return nil
	}
	podcastEpisode.Status = db.PodcastEpisodeStatusDownloading
	p.db.Save(&podcastEpisode)
	// nolint: bodyclose
	resp, err := http.Get(podcastEpisode.AudioURL)
	if err != nil {
		return fmt.Errorf("fetch podcast audio: %w", err)
	}
	filename, ok := getContentDispositionFilename(resp.Header.Get("content-disposition"))
	if !ok {
		audioURL, err := url.Parse(podcastEpisode.AudioURL)
		if err != nil {
			return fmt.Errorf("parse podcast audio url: %w", err)
		}
		filename = path.Base(audioURL.Path)
	}
	filename = p.findUniqueEpisodeName(&podcast, &podcastEpisode, filename)
	audioFile, err := os.Create(path.Join(podcast.Fullpath(p.baseDir), filename))
	if err != nil {
		return fmt.Errorf("create audio file: %w", err)
	}
	podcastEpisode.Filename = filename
	sanTitle := strings.ReplaceAll(podcast.Title, "/", "_")
	podcastEpisode.Path = path.Join(sanTitle, filename)
	p.db.Save(&podcastEpisode)
	go func() {
		if err := p.doPodcastDownload(&podcastEpisode, audioFile, resp.Body); err != nil {
			log.Printf("error downloading podcast: %v", err)
		}
	}()
	return nil
}

func (p *Podcasts) findUniqueEpisodeName(
	podcast *db.Podcast,
	podcastEpisode *db.PodcastEpisode,
	filename string) string {
	podcastPath := path.Join(podcast.Fullpath(p.baseDir), filename)
	if _, err := os.Stat(podcastPath); os.IsNotExist(err) {
		return filename
	}
	sanitizedTitle := strings.ReplaceAll(podcastEpisode.Title, "/", "_")
	titlePath := fmt.Sprintf("%s%s", sanitizedTitle, filepath.Ext(filename))
	podcastPath = path.Join(podcast.Fullpath(p.baseDir), titlePath)
	if _, err := os.Stat(podcastPath); os.IsNotExist(err) {
		return titlePath
	}
	// try to find a filename like FILENAME (1).mp3 incrementing
	return findEpisode(podcast.Fullpath(p.baseDir), filename, 1)
}

func findEpisode(base, filename string, count int) string {
	noExt := strings.TrimSuffix(filename, filepath.Ext(filename))
	testFile := fmt.Sprintf("%s (%d)%s", noExt, count, filepath.Ext(filename))
	podcastPath := path.Join(base, testFile)
	if _, err := os.Stat(podcastPath); os.IsNotExist(err) {
		return testFile
	}
	return findEpisode(base, filename, count+1)
}

func getContentDispositionFilename(header string) (string, bool) {
	_, params, _ := mime.ParseMediaType(header)
	filename, ok := params["filename"]
	return filename, ok
}

func (p *Podcasts) downloadPodcastCover(podPath string, podcast *db.Podcast) error {
	imageURL, err := url.Parse(podcast.ImageURL)
	if err != nil {
		return fmt.Errorf("parse image url: %w", err)
	}
	ext := path.Ext(imageURL.Path)
	resp, err := http.Get(podcast.ImageURL)
	if err != nil {
		return fmt.Errorf("fetch image url: %w", err)
	}
	defer resp.Body.Close()
	if ext == "" {
		contentHeader := resp.Header.Get("content-disposition")
		filename, _ := getContentDispositionFilename(contentHeader)
		ext = path.Ext(filename)
	}
	coverPath := path.Join(podPath, "cover"+ext)
	coverFile, err := os.Create(coverPath)
	if err != nil {
		return fmt.Errorf("creating podcast cover: %w", err)
	}
	defer coverFile.Close()
	if _, err := io.Copy(coverFile, resp.Body); err != nil {
		return fmt.Errorf("writing podcast cover: %w", err)
	}
	podcastPath := filepath.Clean(strings.ReplaceAll(podcast.Title, "/", "_"))
	podcastFilename := fmt.Sprintf("cover%s", ext)
	podcast.ImagePath = path.Join(podcastPath, podcastFilename)
	if err := p.db.Save(podcast).Error; err != nil {
		return fmt.Errorf("save podcast: %w", err)
	}
	return nil
}

func (p *Podcasts) doPodcastDownload(podcastEpisode *db.PodcastEpisode, file *os.File, src io.Reader) error {
	if _, err := io.Copy(file, src); err != nil {
		return fmt.Errorf("writing podcast episode: %w", err)
	}
	defer file.Close()
	stat, _ := file.Stat()
	podcastPath := path.Join(p.baseDir, podcastEpisode.Path)
	podcastTags, err := p.tagger.Read(podcastPath)
	if err != nil {
		log.Printf("error parsing podcast audio: %e", err)
		podcastEpisode.Status = db.PodcastEpisodeStatusError
		p.db.Save(podcastEpisode)
		return nil
	}
	podcastEpisode.Bitrate = podcastTags.Bitrate()
	podcastEpisode.Status = db.PodcastEpisodeStatusCompleted
	podcastEpisode.Length = podcastTags.Length()
	podcastEpisode.Size = int(stat.Size())
	return p.db.Save(podcastEpisode).Error
}

func (p *Podcasts) DeletePodcast(userID, podcastID int) error {
	podcast := db.Podcast{}
	err := p.db.
		Where("id=? AND user_id=?", podcastID, userID).
		First(&podcast).
		Error
	if err != nil {
		return err
	}
	var userCount int
	p.db.
		Model(&db.Podcast{}).
		Where("title=?", podcast.Title).
		Count(&userCount)
	if userCount == 1 {
		// only delete the folder if there are not multiple listeners
		if err = os.RemoveAll(podcast.Fullpath(p.baseDir)); err != nil {
			return fmt.Errorf("delete podcast directory: %w", err)
		}
	}
	err = p.db.
		Where("id=? AND user_id=?", podcastID, userID).
		Delete(db.Podcast{}).
		Error
	if err != nil {
		return fmt.Errorf("delete podcast row: %w", err)
	}
	return nil
}

func (p *Podcasts) DeletePodcastEpisode(podcastEpisodeID int) error {
	episode := db.PodcastEpisode{}
	err := p.db.First(&episode, podcastEpisodeID).Error
	if err != nil {
		return err
	}
	episode.Status = db.PodcastEpisodeStatusDeleted
	p.db.Save(&episode)
	if err := os.Remove(filepath.Join(p.baseDir, episode.Path)); err != nil {
		return err
	}
	return err
}
