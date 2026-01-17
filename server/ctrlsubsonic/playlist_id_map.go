package ctrlsubsonic

import (
	"crypto/md5"
	"encoding/binary"
	"sync"

	"go.senan.xyz/gonic/server/ctrlsubsonic/specid"
)

// playlistIDMap provides bidirectional mapping between playlist paths and spec IDs
type playlistIDMap struct {
	pathToID map[string]int
	idToPath map[int]string
	mu       sync.RWMutex
}

func newPlaylistIDMap() *playlistIDMap {
	return &playlistIDMap{
		pathToID: make(map[string]int),
		idToPath: make(map[int]string),
	}
}

// pathToSpecID converts a playlist path to a specid.Playlist ID
// Uses MD5 hash of the path to generate a deterministic numeric ID
func (m *playlistIDMap) pathToSpecID(path string) specid.ID {
	m.mu.RLock()
	if id, ok := m.pathToID[path]; ok {
		m.mu.RUnlock()
		return specid.ID{Type: specid.Playlist, Value: id}
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring write lock
	if id, ok := m.pathToID[path]; ok {
		return specid.ID{Type: specid.Playlist, Value: id}
	}

	// Generate deterministic ID from path hash
	hash := md5.Sum([]byte(path))
	// Use first 8 bytes of hash as int64, then convert to int
	// Take absolute value to ensure positive
	id := int(binary.BigEndian.Uint64(hash[:8]))
	if id < 0 {
		id = -id
	}
	// Ensure non-zero
	if id == 0 {
		id = 1
	}

	// Handle collisions by finding next available ID
	for {
		if _, exists := m.idToPath[id]; !exists {
			break
		}
		id++
	}

	m.pathToID[path] = id
	m.idToPath[id] = path

	return specid.ID{Type: specid.Playlist, Value: id}
}

// specIDToPath converts a specid.Playlist ID back to a playlist path
func (m *playlistIDMap) specIDToPath(id specid.ID) (string, bool) {
	if id.Type != specid.Playlist {
		return "", false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	path, ok := m.idToPath[id.Value]
	return path, ok
}
