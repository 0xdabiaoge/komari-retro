package filetransfer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/clients"
	agent_runtime "github.com/komari-monitor/komari/web/agent"
)

const (
	defaultTransferChunkSize = int64(25 * 1024 * 1024)
	minTransferChunkSize     = int64(1 * 1024 * 1024)
	maxTransferChunkSize     = int64(128 * 1024 * 1024)
	previewTokenTTL          = 1 * time.Hour
	uploadSessionTTL         = 2 * time.Hour
)

type remoteFileInfo struct {
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	IsDir      bool      `json:"is_dir"`
	IsSymlink  bool      `json:"is_symlink"`
	Size       int64     `json:"size"`
	Mode       string    `json:"mode"`
	ModeOctal  string    `json:"mode_octal"`
	ModifiedAt time.Time `json:"modified_at"`
}

type previewTokenEntry struct {
	UUID      string
	Path      string
	ExpiresAt time.Time
}

type uploadSession struct {
	UploadID     string    `json:"upload_id"`
	UUID         string    `json:"uuid"`
	OwnerSession string    `json:"-"`
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ChunkSize    int64     `json:"chunk_size"`
	ChunkCount   int64     `json:"chunk_count"`
	CreatedAt    time.Time `json:"created_at"`
}

type transferBridge struct {
	TransferID    string
	ClientUUID    string
	TransferToken string
	Direction     string // "download" or "upload"
	DataStream    chan io.ReadCloser
	UploadBody    io.Reader
	UploadLength  int64
	Done          chan struct{}
	DoneOnce      sync.Once
	Claimed       bool
}

func (bridge *transferBridge) finish() { bridge.DoneOnce.Do(func() { close(bridge.Done) }) }

var executeFileOperation = agent_runtime.ExecuteFileOperation

var (
	previewTokensMu  sync.RWMutex
	previewTokens    = make(map[string]previewTokenEntry)
	uploadSessionsMu sync.RWMutex
	uploadSessions   = make(map[string]*uploadSession)
	bridgesMu        sync.RWMutex
	transferBridges  = make(map[string]*transferBridge)
)

func randomHex(length int) string {
	b := make([]byte, length)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func registerBridge(bridge *transferBridge) {
	bridgesMu.Lock()
	defer bridgesMu.Unlock()
	transferBridges[bridge.TransferID] = bridge
}

func removeBridge(id string) {
	bridgesMu.Lock()
	defer bridgesMu.Unlock()
	delete(transferBridges, id)
}

func getBridge(id string) *transferBridge {
	bridgesMu.RLock()
	defer bridgesMu.RUnlock()
	return transferBridges[id]
}

// HandleFileTransfer handles Agent's HTTP stream connection to /api/clients/transfer/:id
func HandleFileTransfer(c *gin.Context) {
	transferID := c.Param("id")
	token := c.Query("token")
	if token == "" {
		token = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	}
	transferToken := c.Query("transfer_token")

	if transferToken == "" {
		transferToken = c.GetHeader("X-Komari-Transfer-Token")
	}

	bridge := getBridge(transferID)
	if bridge == nil {
		c.String(http.StatusNotFound, "transfer session not found or expired")
		return
	}

	// Verify client token
	client, err := clients.GetClientByUUID(bridge.ClientUUID)
	if err != nil || client.Token != token {
		c.String(http.StatusUnauthorized, "unauthorized agent transfer")
		return
	}

	if bridge.TransferToken != transferToken {
		c.String(http.StatusForbidden, "invalid transfer token")
		return
	}
	bridgesMu.Lock()
	claimed := bridge.Claimed
	bridge.Claimed = true
	bridgesMu.Unlock()
	if claimed {
		c.String(http.StatusConflict, "transfer already attached")
		return
	}

	if bridge.Direction == "download" {
		// Agent streams file data up to the server in request body
		select {
		case bridge.DataStream <- c.Request.Body:
			// Wait until browser download completes
			select {
			case <-bridge.Done:
				c.JSON(http.StatusOK, gin.H{"status": "ok"})
			case <-c.Request.Context().Done():
				c.String(http.StatusGatewayTimeout, "download canceled")
			}
		case <-bridge.Done:
			c.String(http.StatusGone, "transfer closed")
		case <-c.Request.Context().Done():
			c.String(http.StatusGatewayTimeout, "agent canceled")
		}
	} else if bridge.Direction == "upload" {
		// Agent requests file data stream from server in response body
		c.Header("Content-Type", "application/octet-stream")
		if bridge.UploadLength > 0 {
			c.Header("Content-Length", strconv.FormatInt(bridge.UploadLength, 10))
		}
		c.Status(http.StatusOK)
		if bridge.UploadBody != nil {
			_, _ = io.CopyN(c.Writer, bridge.UploadBody, bridge.UploadLength)
		}
		bridge.finish()
	} else {
		c.String(http.StatusBadRequest, "unsupported transfer direction")
	}
}

// HandleFilePreviewToken generates a temporary token for viewing files in Office Online or embeds
func HandleFilePreviewToken(c *gin.Context) {
	uuid := c.Param("uuid")
	path := c.Query("path")
	if uuid == "" || path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "uuid and path are required"})
		return
	}

	token := randomHex(24)
	previewTokensMu.Lock()
	previewTokens[token] = previewTokenEntry{
		UUID:      uuid,
		Path:      path,
		ExpiresAt: time.Now().Add(previewTokenTTL),
	}
	previewTokensMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"token": token,
		},
	})
}

// HandleFileDownload handles browser download: GET /api/admin/client/:uuid/file/download
func HandleFileDownload(c *gin.Context) {
	uuid := c.Param("uuid")
	path := c.Query("path")
	inline := c.Query("inline") == "1"
	chunkSizeStr := c.Query("chunk_size")

	streamDownload(c, uuid, path, inline, chunkSizeStr)
}

// HandlePreviewDownload handles preview download: GET /api/preview/client/:uuid/file/download
func HandlePreviewDownload(c *gin.Context) {
	uuid := c.Param("uuid")
	previewToken := c.Query("preview_token")
	inline := true
	chunkSizeStr := c.Query("chunk_size")

	if previewToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "missing preview token"})
		return
	}

	previewTokensMu.RLock()
	entry, exists := previewTokens[previewToken]
	previewTokensMu.RUnlock()

	if !exists || entry.ExpiresAt.Before(time.Now()) || entry.UUID != uuid {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "invalid or expired preview token"})
		return
	}

	streamDownload(c, uuid, entry.Path, inline, chunkSizeStr)
}

func streamDownload(c *gin.Context, uuid, path string, inline bool, chunkSizeStr string) {
	if uuid == "" || path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "uuid and path are required"})
		return
	}

	// 1. Get file stat from agent
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	statRaw, err := executeFileOperation(ctx, uuid, "stat", map[string]interface{}{"path": path})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("Failed to stat file: %v", err)})
		return
	}

	var stat remoteFileInfo
	if err := json.Unmarshal(statRaw, &stat); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to parse file metadata"})
		return
	}

	if stat.IsDir {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Cannot download a directory"})
		return
	}

	if stat.Size < 0 || stat.Size > 1<<40 {
		c.JSON(400, gin.H{"message": "File exceeds the 1 TiB transfer limit"})
		return
	}
	offset, length := int64(0), stat.Size
	if rangeHeader := c.GetHeader("Range"); rangeHeader != "" {
		var rangeErr error
		offset, length, rangeErr = parseRange(rangeHeader, stat.Size)
		if rangeErr != nil {
			c.Header("Content-Range", fmt.Sprintf("bytes */%d", stat.Size))
			c.Status(416)
			return
		}
		c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, offset+length-1, stat.Size))
		c.Status(http.StatusPartialContent)
	}
	c.Header("Accept-Ranges", "bytes")
	if stat.Size == 0 {
		c.Header("Content-Length", "0")
		c.Status(http.StatusOK)
		return
	}
	chunkSize := defaultTransferChunkSize
	if chunkSizeStr != "" {
		if parsed, err := strconv.ParseInt(chunkSizeStr, 10, 64); err == nil && parsed >= minTransferChunkSize && parsed <= maxTransferChunkSize {
			chunkSize = parsed
		}
	}

	transferID := randomHex(20)
	transferToken := randomHex(20)

	bridge := &transferBridge{
		TransferID:    transferID,
		ClientUUID:    uuid,
		TransferToken: transferToken,
		Direction:     "download",
		DataStream:    make(chan io.ReadCloser, 1),
		Done:          make(chan struct{}),
	}
	registerBridge(bridge)
	defer removeBridge(transferID)
	defer bridge.finish()

	// Dispatch download_stream to agent
	streamErrors := make(chan error, 1)
	go func() {
		streamCtx, streamCancel := context.WithTimeout(c.Request.Context(), 2*time.Hour)
		defer streamCancel()
		_, operationErr := executeFileOperation(streamCtx, uuid, "download_stream", map[string]interface{}{
			"path":           path,
			"transfer_id":    transferID,
			"transfer_token": transferToken,
			"offset":         offset,
			"length":         length,
			"file_size":      stat.Size,
		})
		if operationErr != nil {
			streamErrors <- operationErr
		}
	}()

	select {
	case <-c.Request.Context().Done():
		return
	case streamErr := <-streamErrors:
		c.JSON(http.StatusBadGateway, gin.H{"message": streamErr.Error()})
		return
	case <-time.After(30 * time.Second):
		c.JSON(http.StatusGatewayTimeout, gin.H{"message": "Agent stream timeout"})
		return
	case stream := <-bridge.DataStream:
		defer stream.Close()

		ext := filepath.Ext(stat.Name)
		contentType := mime.TypeByExtension(ext)
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		filename := filepath.Base(stat.Path)
		if filename == "." || filename == "/" {
			filename = stat.Name
		}
		if filename == "" {
			filename = "file"
		}

		dispositionType := "attachment"
		if inline && (strings.HasPrefix(contentType, "image/") && contentType != "image/svg+xml" || contentType == "application/pdf" || strings.HasPrefix(contentType, "text/plain")) {
			dispositionType = "inline"
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Content-Security-Policy", "sandbox; default-src 'none'")

		c.Header("Content-Type", contentType)
		c.Header("Content-Length", strconv.FormatInt(length, 10))
		c.Header("Content-Disposition", mime.FormatMediaType(dispositionType, map[string]string{"filename": filename}))
		c.Header("X-Komari-Transfer-Chunk-Size", strconv.FormatInt(chunkSize, 10))
		if !stat.ModifiedAt.IsZero() {
			c.Header("Last-Modified", stat.ModifiedAt.UTC().Format(http.TimeFormat))
		}

		_, _ = io.CopyN(c.Writer, stream, length)
		bridge.finish()
		return
	}
}

// HandleFileUpload handles POST /api/admin/client/:uuid/file/upload
func HandleFileUpload(c *gin.Context) {
	uuid := c.Param("uuid")
	operation := strings.ToLower(strings.TrimSpace(c.Query("operation")))

	switch operation {
	case "init", "start":
		handleUploadInit(c, uuid)
	case "chunk":
		handleUploadChunk(c, uuid)
	case "merge", "commit":
		handleUploadCommit(c, uuid)
	case "cancel":
		handleUploadCancel(c, uuid)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("unsupported upload operation: %s", operation)})
	}
}

func handleUploadInit(c *gin.Context, uuid string) {
	var body struct {
		Path      string `json:"path"`
		Size      int64  `json:"size"`
		ChunkSize int64  `json:"chunk_size"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid init payload"})
		return
	}

	if body.Size < 0 || body.Size > 1<<40 || strings.TrimSpace(body.Path) == "" {
		c.JSON(400, gin.H{"message": "invalid file size or path"})
		return
	}

	chunkSize := body.ChunkSize
	if chunkSize < minTransferChunkSize || chunkSize > maxTransferChunkSize {
		chunkSize = defaultTransferChunkSize
	}

	uploadID := randomHex(20)

	// If empty file, directly invoke create on agent
	if body.Size == 0 {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		defer cancel()
		_, err := executeFileOperation(ctx, uuid, "create", map[string]interface{}{"path": body.Path})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": fmt.Sprintf("Failed to create file: %v", err)})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"upload_id":   uploadID,
				"chunk_size":  chunkSize,
				"chunk_count": 0,
				"complete":    true,
			},
		})
		return
	}

	chunkCount := (body.Size + chunkSize - 1) / chunkSize

	session := &uploadSession{
		UploadID:     uploadID,
		UUID:         uuid,
		OwnerSession: c.GetString("session"),
		Path:         body.Path,
		Size:         body.Size,
		ChunkSize:    chunkSize,
		ChunkCount:   chunkCount,
		CreatedAt:    time.Now(),
	}

	uploadSessionsMu.Lock()
	if len(uploadSessions) >= 1024 {
		uploadSessionsMu.Unlock()
		c.JSON(http.StatusTooManyRequests, gin.H{"message": "Upload session capacity exceeded"})
		return
	}
	uploadSessions[uploadID] = session
	uploadSessionsMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"upload_id":   uploadID,
			"chunk_size":  chunkSize,
			"chunk_count": chunkCount,
		},
	})
}

func handleUploadChunk(c *gin.Context, uuid string) {
	uploadID := c.Query("upload_id")
	chunkIndexStr := c.Query("chunk_index")

	chunkIndex, err := strconv.ParseInt(chunkIndexStr, 10, 64)
	if err != nil || chunkIndex < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid chunk_index"})
		return
	}

	uploadSessionsMu.RLock()
	session, exists := uploadSessions[uploadID]
	uploadSessionsMu.RUnlock()

	if !exists || session.UUID != uuid || session.OwnerSession != c.GetString("session") || time.Since(session.CreatedAt) > uploadSessionTTL {
		c.JSON(http.StatusNotFound, gin.H{"message": "upload session not found or expired"})
		return
	}

	if chunkIndex >= session.ChunkCount {
		c.JSON(400, gin.H{"message": "chunk index out of range"})
		return
	}
	offset := chunkIndex * session.ChunkSize
	length := session.ChunkSize
	if offset+length > session.Size {
		length = session.Size - offset
	}

	transferID := randomHex(20)
	transferToken := randomHex(20)

	bridge := &transferBridge{
		TransferID:    transferID,
		ClientUUID:    uuid,
		TransferToken: transferToken,
		Direction:     "upload",
		UploadBody:    http.MaxBytesReader(c.Writer, c.Request.Body, length),
		UploadLength:  length,
		Done:          make(chan struct{}),
	}
	registerBridge(bridge)
	defer removeBridge(transferID)
	defer bridge.finish()

	args := map[string]interface{}{
		"path":           session.Path,
		"upload_id":      session.UploadID,
		"transfer_id":    transferID,
		"transfer_token": transferToken,
		"offset":         offset,
		"length":         length,
		"chunk_index":    chunkIndex,
		"chunk_count":    session.ChunkCount,
		"total_size":     session.Size,
		"chunk_size":     session.ChunkSize,
		"expected":       length,
		"first":          chunkIndex == 0,
	}

	// Dispatch upload_stream to agent and wait for it
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
	defer cancel()

	_, err = executeFileOperation(ctx, uuid, "upload_stream", args)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": fmt.Sprintf("upload chunk failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "ok"}})
}

func handleUploadCommit(c *gin.Context, uuid string) {
	var body struct {
		UploadID string `json:"upload_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.UploadID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "missing upload_id"})
		return
	}

	uploadSessionsMu.Lock()
	session, exists := uploadSessions[body.UploadID]

	uploadSessionsMu.Unlock()

	if !exists || session.UUID != uuid || session.OwnerSession != c.GetString("session") || time.Since(session.CreatedAt) > uploadSessionTTL {
		c.JSON(http.StatusNotFound, gin.H{"message": "upload session not found or expired"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()

	_, err := executeFileOperation(ctx, uuid, "upload_commit", map[string]interface{}{
		"upload_id":   session.UploadID,
		"path":        session.Path,
		"total_size":  session.Size,
		"chunk_size":  session.ChunkSize,
		"chunk_count": session.ChunkCount,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": fmt.Sprintf("upload commit failed: %v", err)})
		return
	}

	uploadSessionsMu.Lock()
	delete(uploadSessions, body.UploadID)
	uploadSessionsMu.Unlock()
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "ok"}})
}

func handleUploadCancel(c *gin.Context, uuid string) {
	uploadID := c.Query("upload_id")
	if uploadID == "" {
		var body struct {
			UploadID string `json:"upload_id"`
		}
		_ = c.ShouldBindJSON(&body)
		uploadID = body.UploadID
	}

	if uploadID != "" {
		uploadSessionsMu.Lock()
		if entry := uploadSessions[uploadID]; entry != nil && (entry.UUID != uuid || entry.OwnerSession != c.GetString("session")) {
			uploadSessionsMu.Unlock()
			c.Status(404)
			return
		}
		delete(uploadSessions, uploadID)
		uploadSessionsMu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = executeFileOperation(ctx, uuid, "upload_cancel", map[string]interface{}{
			"upload_id": uploadID,
		})
	}

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "ok"}})
}

func parseRange(header string, size int64) (int64, int64, error) {
	invalid := errors.New("invalid byte range")
	if !strings.HasPrefix(header, "bytes=") || strings.Contains(header, ",") || size <= 0 {
		return 0, 0, invalid
	}
	parts := strings.Split(strings.TrimPrefix(header, "bytes="), "-")
	if len(parts) != 2 {
		return 0, 0, invalid
	}
	if parts[0] == "" {
		n, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, invalid
		}
		if n > size {
			n = size
		}
		return size - n, n, nil
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, invalid
	}
	end := size - 1
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start {
			return 0, 0, invalid
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end - start + 1, nil
}

func init() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for now := range ticker.C {
			previewTokensMu.Lock()
			for id, entry := range previewTokens {
				if now.After(entry.ExpiresAt) {
					delete(previewTokens, id)
				}
			}
			previewTokensMu.Unlock()
			uploadSessionsMu.Lock()
			var expired []*uploadSession
			for id, entry := range uploadSessions {
				if now.Sub(entry.CreatedAt) > uploadSessionTTL {
					expired = append(expired, entry)
					delete(uploadSessions, id)
				}
			}
			uploadSessionsMu.Unlock()
			batchCtx, batchCancel := context.WithTimeout(context.Background(), 30*time.Second)
			for _, entry := range expired {
				if batchCtx.Err() != nil {
					break
				}
				ctx, cancel := context.WithTimeout(batchCtx, 5*time.Second)
				_, _ = executeFileOperation(ctx, entry.UUID, "upload_cancel", map[string]interface{}{"upload_id": entry.UploadID, "path": entry.Path})
				cancel()
			}
			batchCancel()
		}
	}()
}
