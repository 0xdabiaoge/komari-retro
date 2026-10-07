package server

import "time"

// Clean only paths registered by this process; never scan arbitrary directories.
func cleanupExpiredUploads(now time.Time) {
	uploadChunksMu.Lock()
	var expired []string
	for id, state := range uploadChunks {
		if now.Sub(state.CreatedAt) > 2*time.Hour {
			expired = append(expired, id)
		}
	}
	uploadChunksMu.Unlock()
	for _, id := range expired {
		writeLock := uploadWriteLock(id)
		writeLock.Lock()
		uploadLifecycleMu.Lock()
		lifecycle := uploadLifecycles[id]
		active := lifecycle != nil && lifecycle.active > 0
		uploadLifecycleMu.Unlock()
		if active {
			writeLock.Unlock()
			continue
		}
		uploadChunksMu.Lock()
		state, exists := uploadChunks[id]
		if exists && now.Sub(state.CreatedAt) > 2*time.Hour {
			// Keep registration on an I/O error so cleanup can retry next time.
			if err := removeUploadFileLocked(id); err == nil {
				uploadChunksMu.Unlock()
				cancelUploadStreams(id)
				writeLock.Unlock()
				continue
			}
		}
		uploadChunksMu.Unlock()
		writeLock.Unlock()
	}
}

func init() {
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for now := range ticker.C {
			cleanupExpiredUploads(now)
		}
	}()
}
