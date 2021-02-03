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
	"go.senan.xyz/gonic/server/ctrlsubsonic/spec"
	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
	"go.senan.xyz/gonic/server/db"
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
)

func (p *Podcasts) GetAllPodcasts(userID int, includeEpisodes bool) (*spec.Podcasts, error) {
	podcasts := []*db.Podcast{}
	err := p.DB.Where("user_id=?", userID).Order("").Find(&podcasts).Error
	if err != nil {
		return nil, err
	}
	channels := []spec.PodcastChannel{}
	for _, c := range podcasts {
		channel := spec.PodcastChannel{
			ID:               *c.SID(),
			OriginalImageURL: c.ImageURL,
			Title:            c.Title,
			Description:      c.Description,
			URL:              c.URL,
			Status:           episodeSkipped,
		}
		if includeEpisodes {
			channel.Episode, err = p.GetPodcastEpisodes(*c.SID())
			if err != nil {
				return nil, err
			}
		}
		channels = append(channels, channel)
	}
	return &spec.Podcasts{List: channels}, nil
}

func (p *Podcasts) GetPodcast(podcastID, userID int, includeEpisodes bool) (*spec.Podcasts, error) {
	podcasts := []*db.Podcast{}
	err := p.DB.Where("user_id=? AND id=?", userID, podcastID).
		Order("title DESC").
		Find(&podcasts).Error
	if err != nil {
		return nil, err
	}

	channels := []spec.PodcastChannel{}
	for _, c := range podcasts {
		channel := spec.PodcastChannel{
			ID:               *c.SID(),
			OriginalImageURL: c.ImageURL,
			CoverArt:         *c.SID(),
			Title:            c.Title,
			Description:      c.Description,
			URL:              c.URL,
			Status:           episodeSkipped,
		}
		if includeEpisodes {
			channel.Episode, err = p.GetPodcastEpisodes(*c.SID())
			if err != nil {
				return nil, err
			}
		}
		channels = append(channels, channel)
	}
	return &spec.Podcasts{List: channels}, nil
}

func (p *Podcasts) GetPodcastEpisodes(podcastID specid.ID) ([]spec.PodcastEpisode, error) {
	dbEpisodes := []*db.PodcastEpisode{}
	if err := p.DB.
		Where("podcast_id=?", podcastID.Value).
		Order("publish_date DESC").
		Find(&dbEpisodes).Error; err != nil {
		return nil, err
	}
	episodes := []spec.PodcastEpisode{}
	for _, dbe := range dbEpisodes {
		episodes = append(episodes, spec.PodcastEpisode{
			ID:          *dbe.SID(),
			StreamID:    *dbe.SID(),
			ContentType: dbe.MIME(),
			ChannelID:   podcastID,
			Title:       dbe.Title,
			Description: dbe.Description,
			Status:      dbe.Status,
			CoverArt:    podcastID,
			PublishDate: *dbe.PublishDate,
			Genre:       "Podcast",
			Duration:    dbe.Length,
			Year:        dbe.PublishDate.Year(),
			Suffix:      dbe.Ext(),
			BitRate:     dbe.Bitrate,
			IsDir:       false,
			Path:        dbe.Path,
			Size:        dbe.Size,
		})
	}

	return episodes, nil
}

func (p *Podcasts) AddNewPodcast(feed *gofeed.Feed, userID int) (*db.Podcast, error) {
	podcast := db.Podcast{
		Description: feed.Description,
		ImageURL:    feed.Image.URL,
		UserID:      userID,
		Title:       feed.Title,
		URL:         feed.FeedLink,
	}
	podPath := podcast.Fullpath(p.PodcastBasePath)
	err := os.Mkdir(podPath, 0755)
	if err != nil && !os.IsExist(err) {
		return nil, err
	}
	if err := p.DB.Save(&podcast).Error; err != nil {
		return &podcast, err
	}
	if err := p.AddNewEpisodes(userID, podcast.ID, feed.Items); err != nil {
		return nil, err
	}
	go p.downloadPodcastCover(podPath, &podcast)

	return &podcast, nil
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

func (p *Podcasts) AddNewEpisodes(userID int, podcastID int, items []*gofeed.Item) error {
	podcastEpisode := db.PodcastEpisode{}
	err := p.DB.
		Where("podcast_id=?", podcastID).
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
			if err := p.AddEpisode(podcastID, item); err != nil {
				return err
			}
		}
		return nil
	}
	for _, item := range getEntriesAfterDate(items, *podcastEpisode.PublishDate) {
		if err := p.AddEpisode(podcastID, item); err != nil {
			return err
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

func (p *Podcasts) AddEpisode(podcastID int, item *gofeed.Item) error {
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

	for _, enc := range item.Enclosures {
		if !strings.HasPrefix(enc.Type, "audio") {
			continue
		}
		size, _ := strconv.Atoi(enc.Length)
		podcastEpisode := db.PodcastEpisode{
			PodcastID:   podcastID,
			Description: item.Description,
			Title:       item.Title,
			Length:      duration,
			Size:        size,
			PublishDate: item.PublishedParsed,
			AudioURL:    enc.URL,
			Status:      episodeSkipped,
		}
		if err := p.DB.Save(&podcastEpisode).Error; err != nil {
			return err
		}
	}
	return nil
}

func (p *Podcasts) RefreshPodcasts(userID int, serverWide bool) error {
	podcasts := []*db.Podcast{}
	var err error
	if serverWide {
		err = p.DB.Find(&podcasts).Error
	} else {
		err = p.DB.Where("user_id=?", userID).Find(&podcasts).Error
	}
	if err != nil {
		return err
	}

	for _, podcast := range podcasts {
		fp := gofeed.NewParser()
		feed, err := fp.ParseURL(podcast.URL)
		if err != nil {
			log.Printf("Error refreshing podcast with url %s: %s", podcast.URL, err)
			continue
		}
		err = p.AddNewEpisodes(userID, podcast.ID, feed.Items)
		if err != nil {
			log.Printf("Error adding episodes: %s", err)
		}
	}
	return nil
}

func (p *Podcasts) DownloadEpisode(episodeID int) error {
	podcastEpisode := db.PodcastEpisode{}
	podcast := db.Podcast{}
	err := p.DB.Where("id=?", episodeID).First(&podcastEpisode).Error
	if err != nil {
		return err
	}
	err = p.DB.Where("id=?", podcastEpisode.PodcastID).First(&podcast).Error
	if err != nil {
		return err
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
		return err
	}
	filename, ok := getContentDispositionFilename(resp.Header.Get("content-disposition"))
	if !ok {
		audioURL, err := url.Parse(podcastEpisode.AudioURL)
		if err != nil {
			return err
		}
		filename = path.Base(audioURL.Path)
	}
	filename = p.findUniqueEpisodeName(&podcast, &podcastEpisode, filename)
	audioFile, err := os.Create(path.Join(podcast.Fullpath(p.PodcastBasePath), filename))
	if err != nil {
		return err
	}
	podcastEpisode.Filename = filename
	podcastEpisode.Path = path.Join(filepath.Clean(podcast.Title), filename)
	p.DB.Save(&podcastEpisode)
	go p.doPodcastDownload(&podcastEpisode, audioFile, resp.Body)
	return nil
}

func (p *Podcasts) findUniqueEpisodeName(
	podcast *db.Podcast,
	podcastEpisode *db.PodcastEpisode,
	filename string) string {
	fp := path.Join(podcast.Fullpath(p.PodcastBasePath), filename)
	if _, err := os.Stat(fp); os.IsNotExist(err) {
		return filename
	}
	titlePath := fmt.Sprintf("%s%s", podcastEpisode.Title,
		filepath.Ext(filename))
	fp = path.Join(podcast.Fullpath(p.PodcastBasePath), titlePath)
	if _, err := os.Stat(fp); os.IsNotExist(err) {
		return titlePath
	}
	// try to find a filename like FILENAME (1).mp3 incrementing
	return findEpisode(podcast.Fullpath(p.PodcastBasePath), filename, 1)
}

func findEpisode(base, filename string, count int) string {
	testFile := fmt.Sprintf("%s (%d)%s", filename, count, filepath.Ext(filename))
	fp := path.Join(base, testFile)
	if _, err := os.Stat(fp); os.IsNotExist(err) {
		return testFile
	}
	return findEpisode(base, filename, count+1)
}

func getContentDispositionFilename(header string) (string, bool) {
	_, params, _ := mime.ParseMediaType(header)
	filename, ok := params["filename"]
	return filename, ok
}

func (p *Podcasts) downloadPodcastCover(podPath string, podcast *db.Podcast) {
	imageURL, err := url.Parse(podcast.ImageURL)
	if err != nil {
		return
	}
	ext := path.Ext(imageURL.Path)
	resp, err := http.Get(podcast.ImageURL)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if ext == "" {
		filename, _ := getContentDispositionFilename(resp.Header.Get("content-disposition"))
		ext = path.Ext(filename)
	}
	coverPath := path.Join(podPath, "cover"+ext)
	coverFile, err := os.Create(coverPath)
	if err != nil {
		log.Printf("Error creating podcast cover: %s", err)
		return
	}
	if _, err := io.Copy(coverFile, resp.Body); err != nil {
		log.Printf("Error while writing cover: %s", err)
		return
	}
	podcast.ImagePath = path.Join(filepath.Clean(podcast.Title), "cover"+ext)
	p.DB.Save(podcast)
}

func (p *Podcasts) doPodcastDownload(podcastEpisode *db.PodcastEpisode, pdFile *os.File, src io.Reader) {
	_, err := io.Copy(pdFile, src)
	if err != nil {
		log.Printf("Error while writing podcast episode: %s", err)
		podcastEpisode.Status = "error"
		p.DB.Save(podcastEpisode)
		return
	}
	defer pdFile.Close()
	stat, _ := pdFile.Stat()
	podTags, err := tags.New(path.Join(p.PodcastBasePath, podcastEpisode.Path))
	if err != nil {
		log.Printf("Error parsing podcast: %e", err)
		podcastEpisode.Status = "error"
		p.DB.Save(podcastEpisode)
		return
	}
	podcastEpisode.Bitrate = podTags.Bitrate()
	podcastEpisode.Status = "completed"
	podcastEpisode.Length = podTags.Length()
	podcastEpisode.Size = int(stat.Size())
	p.DB.Save(podcastEpisode)
}

func (p *Podcasts) DeletePodcast(userID, podcastID int) error {
	podcast := db.Podcast{}
	err := p.DB.Where("id=? AND user_id=?", podcastID, userID).First(&podcast).Error
	if err != nil {
		return err
	}
	userCount := 0
	p.DB.Model(&db.Podcast{}).Where("title=?", podcast.Title).Count(&userCount)
	if userCount == 1 {
		// only delete the folder if there are not multiple listeners
		err = os.RemoveAll(podcast.Fullpath(p.PodcastBasePath))
		if err != nil {
			return err
		}
	}
	err = p.DB.
		Where("id=? AND user_id=?", podcastID, userID).
		Delete(db.Podcast{}).Error
	if err != nil {
		return err
	}
	return nil
}

func (p *Podcasts) DeletePodcastEpisode(podcastEpisodeID int) error {
	podcastEp := db.PodcastEpisode{}
	err := p.DB.First(&podcastEp, podcastEpisodeID).Error
	if err != nil {
		return err
	}
	podcastEp.Status = episodeDeleted
	p.DB.Save(&podcastEp)
	if err := os.Remove(filepath.Join(p.PodcastBasePath, podcastEp.Path)); err != nil {
		return err
	}
	return err
}
