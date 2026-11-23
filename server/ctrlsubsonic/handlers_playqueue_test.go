package ctrlsubsonic

import (
	"encoding/json"
	"net/url"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/require"

	"go.senan.xyz/gonic/db"
)

type playQueueAPIResponse struct {
	Response struct {
		Status string `json:"status"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		OpenSubsonicExtensions []struct {
			Name     string `json:"name"`
			Versions []int  `json:"versions"`
		} `json:"openSubsonicExtensions"`
		PlayQueue *struct {
			Current   string `json:"current"`
			Position  int    `json:"position"`
			Username  string `json:"username"`
			ChangedBy string `json:"changedBy"`
			Entry     []struct {
				ID string `json:"id"`
			} `json:"entry"`
		} `json:"playQueue"`
		PlayQueueByIndex *struct {
			CurrentIndex int    `json:"currentIndex"`
			Position     int    `json:"position"`
			Username     string `json:"username"`
			ChangedBy    string `json:"changedBy"`
			Entry        []struct {
				ID string `json:"id"`
			} `json:"entry"`
		} `json:"playQueueByIndex"`
	} `json:"subsonic-response"`
}

func TestPlayQueueByIndex(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.seq = true

	id0 := f.trackAB1.SID().String()
	id1 := f.trackVA0.SID().String()

	empty := decodePlayQueueAPI(t, f.query(t, f.contr.ServeGetPlayQueueByIndex, f.admin, url.Values{}))
	require.Equal(t, "ok", empty.Response.Status)
	require.Nil(t, empty.Response.PlayQueueByIndex)

	synctest.Test(t, func(*testing.T) {
		requireOKPlayQueue(t, f.query(t, f.contr.ServeSavePlayQueueByIndex, f.admin, url.Values{
			"id":           {id0, id1, id0},
			"currentIndex": {"2"},
			"position":     {"12345"},
		}))
	})

	got := decodePlayQueueAPI(t, f.query(t, f.contr.ServeGetPlayQueueByIndex, f.admin, url.Values{}))
	require.NotNil(t, got.Response.PlayQueueByIndex)
	require.Equal(t, 2, got.Response.PlayQueueByIndex.CurrentIndex)
	require.Equal(t, 12345, got.Response.PlayQueueByIndex.Position)
	require.Equal(t, f.admin.Name, got.Response.PlayQueueByIndex.Username)
	require.Equal(t, mockClientName, got.Response.PlayQueueByIndex.ChangedBy)
	require.Len(t, got.Response.PlayQueueByIndex.Entry, 3)
	require.Equal(t, id0, got.Response.PlayQueueByIndex.Entry[0].ID)
	require.Equal(t, id1, got.Response.PlayQueueByIndex.Entry[1].ID)
	require.Equal(t, id0, got.Response.PlayQueueByIndex.Entry[2].ID)

	// legacy getPlayQueue should expose the current track id from the index
	legacy := decodePlayQueueAPI(t, f.query(t, f.contr.ServeGetPlayQueue, f.admin, url.Values{}))
	require.NotNil(t, legacy.Response.PlayQueue)
	require.Equal(t, id0, legacy.Response.PlayQueue.Current)
	require.Equal(t, 12345, legacy.Response.PlayQueue.Position)
	require.Len(t, legacy.Response.PlayQueue.Entry, 3)

	var queue db.PlayQueue
	require.NoError(t, f.dbc.Where("user_id=?", f.admin.ID).First(&queue).Error)
	require.Equal(t, 2, queue.CurrentIndex)
	require.Equal(t, id0, queue.Current)

	requireErrorCodePlayQueue(t, f.query(t, f.contr.ServeSavePlayQueueByIndex, f.admin, url.Values{
		"id":           {id0},
		"currentIndex": {"5"},
	}), 10)

	// clearing must reject currentIndex and remove the saved queue
	requireErrorCodePlayQueue(t, f.query(t, f.contr.ServeSavePlayQueueByIndex, f.admin, url.Values{
		"currentIndex": {"0"},
	}), 10)

	synctest.Test(t, func(*testing.T) {
		requireOKPlayQueue(t, f.query(t, f.contr.ServeSavePlayQueueByIndex, f.admin, url.Values{}))
	})

	cleared := decodePlayQueueAPI(t, f.query(t, f.contr.ServeGetPlayQueueByIndex, f.admin, url.Values{}))
	require.Equal(t, "ok", cleared.Response.Status)
	require.Nil(t, cleared.Response.PlayQueueByIndex)
	require.Error(t, f.dbc.Where("user_id=?", f.admin.ID).First(&db.PlayQueue{}).Error)
}

func TestPlayQueueLegacySaveSetsIndex(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	id0 := f.trackAB1.SID().String()
	id1 := f.trackVA0.SID().String()

	synctest.Test(t, func(*testing.T) {
		requireOKPlayQueue(t, f.query(t, f.contr.ServeSavePlayQueue, f.admin, url.Values{
			"id":       {id0, id1, id0},
			"current":  {id0},
			"position": {"999"},
		}))
	})

	// current points at the first occurrence when using legacy current=id
	got := decodePlayQueueAPI(t, f.query(t, f.contr.ServeGetPlayQueueByIndex, f.admin, url.Values{}))
	require.NotNil(t, got.Response.PlayQueueByIndex)
	require.Equal(t, 0, got.Response.PlayQueueByIndex.CurrentIndex)
	require.Equal(t, 999, got.Response.PlayQueueByIndex.Position)
	require.Len(t, got.Response.PlayQueueByIndex.Entry, 3)

	synctest.Test(t, func(*testing.T) {
		requireOKPlayQueue(t, f.query(t, f.contr.ServeSavePlayQueue, f.admin, url.Values{}))
	})
	require.Error(t, f.dbc.Where("user_id=?", f.admin.ID).First(&db.PlayQueue{}).Error)
}

func TestOpenSubsonicExtensionsIncludesIndexBasedQueue(t *testing.T) {
	t.Parallel()
	f := newFixture(t)

	body := decodePlayQueueAPI(t, f.query(t, f.contr.ServeGetOpenSubsonicExtensions, f.admin, url.Values{}))
	found := false
	for _, ext := range body.Response.OpenSubsonicExtensions {
		if ext.Name == "indexBasedQueue" {
			found = true
			require.Equal(t, []int{1}, ext.Versions)
		}
	}
	require.True(t, found, "indexBasedQueue extension missing")
}

func decodePlayQueueAPI(t *testing.T, body string) playQueueAPIResponse {
	t.Helper()
	var sub playQueueAPIResponse
	require.NoError(t, json.Unmarshal([]byte(body), &sub), body)
	return sub
}

func requireOKPlayQueue(t *testing.T, body string) {
	t.Helper()
	sub := decodePlayQueueAPI(t, body)
	require.Equal(t, "ok", sub.Response.Status, body)
}

func requireErrorCodePlayQueue(t *testing.T, body string, code int) {
	t.Helper()
	sub := decodePlayQueueAPI(t, body)
	require.Equal(t, "failed", sub.Response.Status, body)
	require.NotNil(t, sub.Response.Error)
	require.Equal(t, code, sub.Response.Error.Code)
}
