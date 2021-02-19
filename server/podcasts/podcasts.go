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

type Podcasts struct {
	DB              *db.DB
	PodcastBasePath string
}

const (
	episodeDownloading = "downloading"
	episodeSkipped     = "skipped"
	episodeDeleted     = "deleted"
	episodeCompleted   = "completed"

	autoDlLatest = "dl_latest"
	autoDlNone   = "dl_none"
)

func (p *Podcasts) GetPodcastOrAll(userID int, id int, includeEpisodes bool) ([]*db.Podcast, error) {
	podcasts := []*db.Podcast{}
	q := p.DB.Where("user_id=?", userID)
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
	err := p.DB.
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
	podPath := podcast.Fullpath(p.PodcastBasePath)
	err := os.Mkdir(podPath, 0755)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	if err := p.DB.Save(&podcast).Error; err != nil {
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

var errNoSuchAutoDlOption = errors.New("invalid autodownload setting")

func (p *Podcasts) SetAutoDownload(podcastID int, setting string) error {
	podcast := db.Podcast{}
	err := p.DB.
		Where("id=?", podcastID).
		First(&podcast).
		Error
	if err != nil {
		return err
	}
	switch setting {
	case autoDlLatest:
		podcast.AutoDownload = autoDlLatest
		return p.DB.Save(&podcast).Error
	case autoDlNone:
		podcast.AutoDownload = autoDlNone
		return p.DB.Save(&podcast).Error
	}
	return errNoSuchAutoDlOption
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
	err := p.DB.
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
		if podcast.AutoDownload == autoDlLatest &&
			(episode.Status != episodeCompleted && episode.Status != episodeDownloading) {
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
	if duration == 0 {
		duration = getSecondsFromString(item.ITunesExt.Duration)
	}

	if episode, ok := p.findEnclosureAudio(podcastID, duration, item); ok {
		if err := p.DB.Save(episode).Error; err != nil {
			return nil, err
		}
		return episode, nil
	}
	if episode, ok := p.findMediaAudio(podcastID, duration, item); ok {
		if err := p.DB.Save(episode).Error; err != nil {
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
		Status:      episodeSkipped,
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
	if err := p.DB.Find(&podcasts).Error; err != nil {
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
	err := p.DB.
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
	var errs multierr.Err
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
	err := p.DB.
		Where("podcast_id=?", podcastID).
		Find(&podcastEpisodes).
		Error
	if err != nil {
		return fmt.Errorf("get episodes by podcast id: %w", err)
	}
	go func() {
		for _, episode := range podcastEpisodes {
			if episode.Status == episodeDownloading || episode.Status == episodeCompleted {
				log.Println("skipping episode is in progress or already downloaded")
				continue
			}
			if err := p.DownloadEpisode(episode.ID); err != nil {
				log.Println(err)
			}
			log.Printf("Finished downloading episode: \"%s\"", episode.Title)
		}
	}()
	return nil
}

func (p *Podcasts) DownloadEpisode(episodeID int) error {
	podcastEpisode := db.PodcastEpisode{}
	podcast := db.Podcast{}
	err := p.DB.
		Where("id=?", episodeID).
		First(&podcastEpisode).
		Error
	if err != nil {
		return fmt.Errorf("get podcast episode by id: %w", err)
	}
	err = p.DB.
		Where("id=?", podcastEpisode.PodcastID).
		First(&podcast).
		Error
	if err != nil {
		return fmt.Errorf("get podcast by id: %w", err)
	}
	if podcastEpisode.Status == episodeDownloading {
		log.Printf("Already downloading podcast episode with id %d", episodeID)
		return nil
	}
	podcastEpisode.Status = episodeDownloading
	p.DB.Save(&podcastEpisode)
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
	audioFile, err := os.Create(path.Join(podcast.Fullpath(p.PodcastBasePath), filename))
	if err != nil {
		return fmt.Errorf("create audio file: %w", err)
	}
	podcastEpisode.Filename = filename
	sanTitle := strings.ReplaceAll(podcast.Title, "/", "_")
	podcastEpisode.Path = path.Join(sanTitle, filename)
	p.DB.Save(&podcastEpisode)
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
	podcastPath := path.Join(podcast.Fullpath(p.PodcastBasePath), filename)
	if _, err := os.Stat(podcastPath); os.IsNotExist(err) {
		return filename
	}
	sanitizedTitle := strings.ReplaceAll(podcastEpisode.Title, "/", "_")
	titlePath := fmt.Sprintf("%s%s", sanitizedTitle, filepath.Ext(filename))
	podcastPath = path.Join(podcast.Fullpath(p.PodcastBasePath), titlePath)
	if _, err := os.Stat(podcastPath); os.IsNotExist(err) {
		return titlePath
	}
	// try to find a filename like FILENAME (1).mp3 incrementing
	return findEpisode(podcast.Fullpath(p.PodcastBasePath), filename, 1)
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
	if _, err := io.Copy(coverFile, resp.Body); err != nil {
		return fmt.Errorf("writing podcast cover: %w", err)
	}
	podcastPath := filepath.Clean(podcast.Title)
	podcastFilename := fmt.Sprintf("cover%s", ext)
	podcast.ImagePath = path.Join(podcastPath, podcastFilename)
	if err := p.DB.Save(podcast).Error; err != nil {
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
	podcastPath := path.Join(p.PodcastBasePath, podcastEpisode.Path)
	podcastTags, err := tags.New(podcastPath)
	if err != nil {
		log.Printf("error parsing podcast audio: %e", err)
		podcastEpisode.Status = "error"
		p.DB.Save(podcastEpisode)
		return nil
	}
	podcastEpisode.Bitrate = podcastTags.Bitrate()
	podcastEpisode.Status = episodeCompleted
	podcastEpisode.Length = podcastTags.Length()
	podcastEpisode.Size = int(stat.Size())
	return p.DB.Save(podcastEpisode).Error
}

func (p *Podcasts) DeletePodcast(userID, podcastID int) error {
	podcast := db.Podcast{}
	err := p.DB.
		Where("id=? AND user_id=?", podcastID, userID).
		First(&podcast).
		Error
	if err != nil {
		return err
	}
	var userCount int
	p.DB.
		Model(&db.Podcast{}).
		Where("title=?", podcast.Title).
		Count(&userCount)
	if userCount == 1 {
		// only delete the folder if there are not multiple listeners
		if err = os.RemoveAll(podcast.Fullpath(p.PodcastBasePath)); err != nil {
			return fmt.Errorf("delete podcast directory: %w", err)
		}
	}
	err = p.DB.
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
	err := p.DB.First(&episode, podcastEpisodeID).Error
	if err != nil {
		return err
	}
	episode.Status = episodeDeleted
	p.DB.Save(&episode)
	if err := os.Remove(filepath.Join(p.PodcastBasePath, episode.Path)); err != nil {
		return err
	}
	return err
}
