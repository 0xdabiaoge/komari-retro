package admin

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/web/api"
)

// 只有一个备份恢复操作在进行
var restoreMutex sync.Mutex

// UploadBackup 用于接收上传的备份文件并将其内容恢复到原始位置
func UploadBackup(c *gin.Context) {
	if !dbcore.RestoreSupported() {
		api.RespondError(c, 400, "Restore requires the standard ./data/komari.db database path")
		return
	}
	// 尝试获取锁，如果已有恢复操作在进行，则立即返回错误
	if !restoreMutex.TryLock() {
		api.RespondError(c, http.StatusConflict, "Another restore operation is already in progress")
		return
	}
	defer restoreMutex.Unlock()

	// 获取上传的文件
	file, header, err := c.Request.FormFile("backup")
	if err != nil {
		api.RespondError(c, http.StatusBadRequest, fmt.Sprintf("Error getting uploaded file: %v", err))
		return
	}
	defer file.Close()

	// 检查文件是否为zip格式
	if !strings.HasSuffix(strings.ToLower(header.Filename), ".zip") {
		api.RespondError(c, http.StatusBadRequest, "Uploaded file must be a ZIP archive")
		return
	}

	// 确保data目录存在
	if err := os.MkdirAll("./data", 0755); err != nil {
		api.RespondError(c, http.StatusInternalServerError, fmt.Sprintf("Error creating data directory: %v", err))
		return
	}

	// 创建临时文件保存上传的zip（先校验，再落地到固定位置）
	tempFile, err := os.CreateTemp("./data", ".backup-upload-*.zip")
	if err != nil {
		api.RespondError(c, http.StatusInternalServerError, fmt.Sprintf("Error creating temporary file: %v", err))
		return
	}
	tempFilePath := tempFile.Name()
	defer os.Remove(tempFilePath) // 确保临时文件最终被删除

	// 将上传的文件内容复制到临时文件
	_, err = io.Copy(tempFile, file)
	if err != nil {
		tempFile.Close()
		api.RespondError(c, http.StatusInternalServerError, fmt.Sprintf("Error saving uploaded file: %v", err))
		return
	}
	tempFile.Close() // 关闭文件以便后续操作

	if err := dbcore.ValidateBackup(tempFilePath); err != nil {
		api.RespondError(c, 400, "Invalid backup: "+err.Error())
		return
	}
	finalPath := filepath.Join("data", "backup.zip")
	if _, err := os.Stat(finalPath); err == nil {
		api.RespondError(c, 409, "A backup is already pending")
		return
	}
	if err := os.Rename(tempFilePath, finalPath); err != nil {
		api.RespondError(c, 500, "Unable to queue backup")
		return
	}

	// 返回：已保存备份，重启后将自动恢复
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": "Backup uploaded successfully. The service will restart and apply the backup.",
		"path":    "./data/backup.zip",
	})

	go func() {
		log.Println("Backup uploaded, restarting service in 2 seconds to apply on startup...")
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()
}
