package log

import (
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// GinLogger 返回一个 gin.HandlerFunc，用于记录 HTTP 请求
func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := safeLogPath(c.Request.URL.Path)
		query := safeLogQuery(c.Request.URL.Query())

		// 处理请求
		c.Next()

		// 计算延迟
		latency := time.Since(start)

		// 获取状态码
		statusCode := c.Writer.Status()

		statusCodeColored := func(code int) string {
			if code >= 500 {
				return Red("%d", code)
			} else if code >= 400 {
				return Yellow("%d", code)
			} else if code >= 300 {
				return Cyan("%d", code)
			} else {
				return Green("%d", code)
			}
		}(statusCode)

		msg := fmt.Sprintf("%s %s %s", statusCodeColored, c.Request.Method, path)
		if query != "" {
			msg += "?" + query
		}
		msg += fmt.Sprintf(" | %s | %s", c.ClientIP(), latency)

		if len(c.Errors) > 0 {
			msg += " | " + c.Errors.String()
		}

		handler := slog.Default().Handler()
		var level slog.Level
		if statusCode >= 500 {
			level = slog.LevelError
		} else {
			level = slog.LevelInfo
		}

		r := slog.NewRecord(time.Now(), level, msg, 0)
		r.AddAttrs(slog.String("_group", "GIN")) // 添加分组标识
		handler.Handle(c.Request.Context(), r)
	}
}

// Only non-secret routing and pagination fields are useful in access logs.
func safeLogQuery(values url.Values) string {
	clean := make(url.Values)
	for _, key := range []string{"page", "limit", "hours", "operation", "chunk_index"} {
		if value := values.Get(key); value != "" && len(value) <= 32 {
			clean.Set(key, value)
		}
	}
	return clean.Encode()
}

func safeLogPath(path string) string {
	if strings.HasPrefix(path, "/s/") {
		return "/s/[redacted]"
	}
	return path
}

// GinRecovery 返回一个 gin.HandlerFunc，用于恢复 panic
func GinRecovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				msg := fmt.Sprintf("panic recovered: %v | %s %s", err, c.Request.Method, safeLogPath(c.Request.URL.Path))
				handler := slog.Default().Handler()
				r := slog.NewRecord(time.Now(), slog.LevelError, msg, 0)
				r.AddAttrs(slog.String("_group", "GIN"))
				handler.Handle(c.Request.Context(), r)
				c.AbortWithStatus(500)
			}
		}()
		c.Next()
	}
}
