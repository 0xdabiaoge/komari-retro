package public

import (
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/komari-monitor/komari/database/accounts"
	"github.com/komari-monitor/komari/pkg/config"
)

// Next.js stores its exported theme assets under `_next`, which `go:embed`
// skips when recursively embedding a directory unless the subtree uses `all:`.
//go:embed defaultTheme retroTheme all:retroTheme/dist/_next
var PublicFS embed.FS

// 常量定义
const (
	DataDir            = "./data"
	ThemesDir          = "theme"
	FaviconFile        = "favicon.ico"
	DefaultTheme       = "default"
	RetroTheme         = "retro"
	LanguageCookieName = "language"

	// 主题内部结构定义
	DistDir   = "dist"       // 静态资源存放目录
	IndexFile = "index.html" // 相对于 DistDir
)

var builtinThemeDirs = map[string]string{
	DefaultTheme: "defaultTheme",
	RetroTheme:   "retroTheme",
}

// BuiltinThemeIDs returns the built-in themes in display order.
func BuiltinThemeIDs() []string {
	return []string{DefaultTheme, RetroTheme}
}

// IsBuiltinTheme reports whether short identifies an embedded theme.
func IsBuiltinTheme(short string) bool {
	_, ok := builtinThemeDirs[strings.ToLower(strings.TrimSpace(short))]
	return ok
}

// ReadBuiltinThemeFile reads a file relative to an embedded theme root.
func ReadBuiltinThemeFile(short, relativePath string) ([]byte, bool) {
	root, ok := builtinThemeDirs[strings.ToLower(strings.TrimSpace(short))]
	if !ok {
		return nil, false
	}

	cleanPath := path.Clean(strings.ReplaceAll(relativePath, "\\", "/"))
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return nil, false
	}

	content, err := fs.ReadFile(PublicFS, path.Join(root, cleanPath))
	return content, err == nil
}

func init() {
	_ = os.MkdirAll("./data/theme", 0755)
	_ = mime.AddExtensionType(".js", "application/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".css", "text/css; charset=utf-8")
}

func getFallbackMimeType(p string) string {
	switch strings.ToLower(filepath.Ext(p)) {
	case ".js", ".mjs":
		return "application/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".svg":
		return "image/svg+xml"
	case ".json":
		return "application/json"
	case ".ico":
		return "image/x-icon"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".webp":
		return "image/webp"
	default:
		return ""
	}
}

func normalizeHTMLLanguage(language string) string {
	language = strings.TrimSpace(strings.ReplaceAll(language, "_", "-"))
	if len(language) < 2 || len(language) > 32 {
		return ""
	}

	for _, r := range language {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return ""
	}

	return language
}

func replaceHTMLLanguage(htmlStr, language string) string {
	language = normalizeHTMLLanguage(language)
	if language == "" {
		return htmlStr
	}

	replacements := []struct {
		old string
		new string
	}{
		{`<html lang="en">`, `<html lang="` + language + `">`},
		{`<html lang='en'>`, `<html lang='` + language + `'>`},
		{`<html>`, `<html lang="` + language + `">`},
	}

	for _, replacement := range replacements {
		if strings.Contains(htmlStr, replacement.old) {
			return strings.Replace(htmlStr, replacement.old, replacement.new, 1)
		}
	}

	return htmlStr
}

// isSafePath 验证路径是否在指定的基础目录内，防止路径穿透攻击
func isSafePath(basePath, targetPath string) bool {
	// 获取基础目录的绝对路径
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return false
	}

	// 清理目标路径，移除 ../ 等
	cleanTarget := filepath.Clean(targetPath)

	// 拼接完整路径
	fullPath := filepath.Join(absBase, cleanTarget)

	// 获取绝对路径
	absTarget, err := filepath.Abs(fullPath)
	if err != nil {
		return false
	}

	// 检查目标路径是否以基础路径开头
	// 使用 filepath.Rel 更可靠地检查路径关系
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false
	}

	// 如果相对路径以 .. 开头，说明目标在基础目录之外
	return !strings.HasPrefix(rel, "..") && rel != ".."
}

// Static 注册静态资源和 SPA 路由处理
func Static(r *gin.RouterGroup, noRoute func(handlers ...gin.HandlerFunc)) {
	// 初始化各内置主题的嵌入式文件系统。
	embeddedThemes := make(map[string]fs.FS, len(builtinThemeDirs))
	for themeID, root := range builtinThemeDirs {
		themeFS, err := fs.Sub(PublicFS, root)
		if err != nil {
			panic("missing embedded theme filesystem: " + themeID)
		}
		embeddedThemes[themeID] = themeFS
		if _, err := fs.Stat(themeFS, path.Join(DistDir, IndexFile)); err != nil {
			panic("missing embedded theme entry point: " + themeID)
		}
	}

	getConfig := func() map[string]any {
		cfg, _ := config.GetMany(map[string]any{
			config.DescriptionKey: "A simple server monitor tool.",
			config.CustomHeadKey:  "",
			config.CustomBodyKey:  "",
			config.SitenameKey:    "Komari Monitor",
			config.ThemeKey:       DefaultTheme,
		})
		return cfg
	}

	// 核心逻辑：获取文件内容
	// filePath: 相对于主题根目录的路径 (例如 "theme.json" 或 "dist/assets/a.js")
	// 返回: content, contentType, exists
	getFileContent := func(themeID string, relativePath string) ([]byte, string, bool) {
		if strings.EqualFold(themeID, "next") || strings.EqualFold(themeID, "nasdaq") || strings.HasPrefix(themeID, ".") {
			themeID = DefaultTheme
		}
		cleanPath := strings.TrimPrefix(relativePath, "/")

		cleanPath = filepath.Clean(cleanPath)

		if IsBuiltinTheme(themeID) {
			if content, ok := ReadBuiltinThemeFile(themeID, cleanPath); ok {
				mimeType := mime.TypeByExtension(filepath.Ext(cleanPath))
				if mimeType == "" {
					mimeType = getFallbackMimeType(cleanPath)
				}
				return content, mimeType, true
			}
			themeID = DefaultTheme
		} else if themeID != DefaultTheme {
			if strings.Contains(themeID, "..") || strings.Contains(themeID, "/") || strings.Contains(themeID, "\\") {
				return nil, "", false
			}

			themeBasePath := filepath.Join(DataDir, ThemesDir, themeID)

			if !isSafePath(themeBasePath, cleanPath) {
				return nil, "", false
			}

			localPath := filepath.Join(themeBasePath, cleanPath)
			// 检查本地自定义文件是否存在且不是目录
			if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
				content, err := os.ReadFile(localPath)
				if err == nil {
					mimeType := mime.TypeByExtension(filepath.Ext(localPath))
					if mimeType == "" {
						mimeType = getFallbackMimeType(localPath)
					}
					return content, mimeType, true
				}
			}

			// 本地文件不存在，或读取失败 -> 继续向下回退到 defaultTheme
		}

		// 2. 尝试从嵌入式 defaultTheme/{cleanPath} 读取
		// fs.ReadFile 处理 embed 路径时使用 "/"
		embedPath := filepath.ToSlash(cleanPath)

		if strings.Contains(embedPath, "..") {
			return nil, "", false
		}

		if content, err := fs.ReadFile(embeddedThemes[DefaultTheme], embedPath); err == nil {
			mimeType := mime.TypeByExtension(filepath.Ext(embedPath))
			if mimeType == "" {
				mimeType = getFallbackMimeType(embedPath)
			}
			return content, mimeType, true
		}

		return nil, "", false
	}

	// 核心逻辑：渲染 Index.html
	serveIndex := func(c *gin.Context) {
		reqPath := c.Request.URL.Path
		cfg := getConfig()

		currentTheme := cfg[config.ThemeKey].(string)
		shouldReplace := true

		// 特殊页面：强制使用 default 主题，且不进行内容替换
		if strings.HasPrefix(reqPath, "/admin") || strings.HasPrefix(reqPath, "/terminal") || strings.HasPrefix(reqPath, "/manage") {
			currentTheme = DefaultTheme
			shouldReplace = false
		}

		// 获取 dist/index.html (相对于主题根目录)
		targetFile := path.Join(DistDir, IndexFile)
		content, _, exists := getFileContent(currentTheme, targetFile)

		if !exists {
			c.String(http.StatusNotFound, "Index file missing (checked %s/dist/index.html and default).", currentTheme)
			return
		}

		htmlStr := string(content)
		if language, err := c.Cookie(LanguageCookieName); err == nil {
			htmlStr = replaceHTMLLanguage(htmlStr, language)
		}

		// 如果不替换，保留系统内置页面内容，仅同步 html lang。
		if !shouldReplace {
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(htmlStr))
			return
		}

		// 执行 HTML 内容替换
		replacer := strings.NewReplacer(
			"<title>Komari Monitor</title>", "<title>"+cfg[config.SitenameKey].(string)+"</title>",
			"A simple server monitor tool.", cfg[config.DescriptionKey].(string),
			"</head>", cfg[config.CustomHeadKey].(string)+"</head>",
			"</body>", cfg[config.CustomBodyKey].(string)+"</body>",
		)

		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(replacer.Replace(htmlStr)))
	}

	// ================= 路由定义 =================

	// 1. Favicon 优先策略
	r.GET("/favicon.ico", func(c *gin.Context) {
		// 优先：./data/favicon.ico
		localFavicon := filepath.Join(DataDir, FaviconFile)
		if _, err := os.Stat(localFavicon); err == nil {
			c.File(localFavicon)
			return
		}

		// 其次：当前主题的 dist/favicon.ico 或 theme_root/favicon.ico ?
		// 通常构建后的资源在 dist 中，这里假设优先找 dist 内的，如果你的 favicon 在根目录，去掉 DistDir 拼接即可
		cfg := getConfig()
		themeFaviconPath := path.Join(DistDir, FaviconFile)
		content, mimeType, exists := getFileContent(cfg[config.ThemeKey].(string), themeFaviconPath)
		if exists {
			c.Data(http.StatusOK, mimeType, content)
			return
		}

		c.Status(http.StatusNotFound)
	})

	// 2. 静态资源路由 /themes/:id/*path
	// 允许访问 /themes/MyTheme/theme.json 和 /themes/MyTheme/dist/assets/a.js
	r.GET("/themes/:id/*path", func(c *gin.Context) {
		themeID := c.Param("id")
		if strings.EqualFold(themeID, "next") || strings.EqualFold(themeID, "nasdaq") || strings.HasPrefix(themeID, ".") {
			c.Status(404)
			return
		}
		// c.Param("path") 包含了开头的 /，getFileContent 会处理
		filePath := c.Param("path")

		content, mimeType, exists := getFileContent(themeID, filePath)
		if exists {
			c.Data(http.StatusOK, mimeType, content)
			return
		}
		c.Status(http.StatusNotFound)
	})

	// 3. SPA 路由 (noRoute)
	noRoute(func(c *gin.Context) {
		if c.Request.Method != http.MethodGet {
			c.Status(http.StatusNotFound)
			return
		}

		reqPath := c.Request.URL.Path
		cfg := getConfig()
		currentTheme := cfg[config.ThemeKey].(string)

		// 静态资源文件优先检查与下发 (如 /assets/..., /favicon.ico 等)
		distPath := path.Join(DistDir, reqPath)
		content, mimeType, exists := getFileContent(currentTheme, distPath)
		if exists {
			c.Data(http.StatusOK, mimeType, content)
			return
		}

		// 检查当前会话是否为已登录管理员
		isAdmin := false
		if session, _ := c.Cookie("session_token"); session != "" {
			if _, err := accounts.GetUserBySession(session); err == nil {
				isAdmin = true
			}
		}

		// 工作台（/terminal）安全加固：无论是否开启私有站点，工作台页面仅允许已登录的管理员访问
		// 未登录访客或外部扫描探测一律返回 404 伪装不存在
		if reqPath == "/terminal" || strings.HasPrefix(reqPath, "/terminal/") {
			if !isAdmin {
				c.String(http.StatusNotFound, "404 page not found")
				return
			}
		}

		privateSite, _ := config.GetAs[bool](config.PrivateSiteKey, false)
		if !privateSite {
			// 非私有站点：正常下发 index.html 驱动 SPA
			serveIndex(c)
			return
		}

		// --- 开启私有站点保护模式 ---
		adminPath, _ := config.GetAs[string](config.AdminPathKey, "")
		adminPath = strings.TrimSpace(adminPath)
		if adminPath == "" {
			adminPath = config.GetOrGenerateAdminPath()
		}
		if !strings.HasPrefix(adminPath, "/") {
			adminPath = "/" + adminPath
		}

		adminViewPath, _ := config.GetAs[string](config.AdminViewPathKey, "")
		adminViewPath = strings.TrimSpace(adminViewPath)
		if adminViewPath == "" {
			adminViewPath = config.GetOrGenerateAdminViewPath()
		}
		if !strings.HasPrefix(adminViewPath, "/") {
			adminViewPath = "/" + adminViewPath
		}

		cleanPath := strings.TrimRight(reqPath, "/")
		cleanAdminPath := strings.TrimRight(adminPath, "/")
		cleanAdminViewPath := strings.TrimRight(adminViewPath, "/")

		// 1. 访问后台安全登录入口 (例如 /entry-xxxxxx) -> 设置凭证并跳转至 /admin/login
		if cleanAdminPath != "" && cleanPath == cleanAdminPath {
			c.SetCookie("admin_entrance_token", adminPath, 7*86400, "/", "", false, true)
			c.Redirect(http.StatusFound, "/admin/login")
			return
		}

		// 2. 访问管理员查看探针前台地址 (例如 /view-xxxxxx) -> 设置凭证并跳转至探针首页 /
		if cleanAdminViewPath != "" && cleanPath == cleanAdminViewPath {
			c.SetCookie("admin_entrance_token", adminPath, 7*86400, "/", "", false, true)
			c.Redirect(http.StatusFound, "/")
			return
		}

		// 3. 访问对外只读分享链接 /s/:token 或 /s/:token/* (支持永久分享与限时分享)
		if strings.HasPrefix(reqPath, "/s/") {
			tokenPart := strings.TrimPrefix(reqPath, "/s/")
			parts := strings.Split(tokenPart, "/")
			shareToken := parts[0]

			permToken, _ := config.GetAs[string](config.PermanentShareTokenKey, "")
			tempToken, _ := config.GetAs[string](config.TemporyShareTokenKey, "")
			expireAt, _ := config.GetAs[int64](config.TemporyShareTokenExpireAtKey, 0)
			now := time.Now().Unix()

			// 永久分享验证
			if permToken != "" && shareToken == permToken {
				c.SetCookie("temp_key", shareToken, 365*86400*10, "/", "", false, false)
				serveIndex(c)
				return
			}

			// 限时分享验证
			if tempToken != "" && shareToken == tempToken && expireAt >= now {
				expireSeconds := int(expireAt - now)
				if expireSeconds > 0 {
					c.SetCookie("temp_key", shareToken, expireSeconds, "/", "", false, false)
				}
				serveIndex(c)
				return
			}

			// 分享密钥无效或已过期，直接返回 404
			c.String(http.StatusNotFound, "404 page not found")
			return
		}

		// 4. 检查是否持有后台安全入口凭证
		hasAdminEntrance := false
		if entranceToken, _ := c.Cookie("admin_entrance_token"); entranceToken != "" && entranceToken == adminPath {
			hasAdminEntrance = true
		}

		if isAdmin || hasAdminEntrance {
			// 已登录管理员或持有安全入口凭证：允许正常访问前台探针、后台管理、工作台终端及所有前端页面
			serveIndex(c)
			return
		}

		// 5. 持有有效 temp_key 访问机器详情页 /instance/:uuid 或返回专属分享页
		if hasValidTempKey(c) {
			if reqPath == "/" {
				tempKey, _ := c.Cookie("temp_key")
				if tempKey == "" {
					tempKey = c.Query("temp_key")
				}
				if tempKey != "" {
					c.Redirect(http.StatusFound, "/s/"+tempKey)
					return
				}
			}
			if strings.HasPrefix(reqPath, "/instance/") {
				serveIndex(c)
				return
			}
		}

		// 6. 其余任何访问（包括外部未授权访问根路径 /、试探 /admin 等），一律返回 404 伪装不存在
		c.String(http.StatusNotFound, "404 page not found")
	})
}

func hasValidTempKey(c *gin.Context) bool {
	tempKey, _ := c.Cookie("temp_key")
	if tempKey == "" {
		tempKey = c.Query("temp_key")
	}
	if tempKey == "" {
		return false
	}
	permToken, _ := config.GetAs[string](config.PermanentShareTokenKey, "")
	if permToken != "" && tempKey == permToken {
		return true
	}
	expireAt, err := config.GetAs[int64](config.TemporyShareTokenExpireAtKey, 0)
	if err != nil {
		return false
	}
	tempToken, err := config.GetAs[string](config.TemporyShareTokenKey, "")
	if err != nil || tempToken == "" || tempKey != tempToken {
		return false
	}
	return expireAt >= time.Now().Unix()
}
