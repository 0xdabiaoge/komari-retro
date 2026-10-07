package utils

import (
	"github.com/gin-gonic/gin"
	"net"
	"os"
	"strings"
)

// https://github.com/labstack/echo/blob/98ca08e7dd64075b858e758d6693bf9799340756/context.go#L275-L294
func GetScheme(c *gin.Context) string {
	// Can't use `r.Request.URL.Scheme`
	// See: https://groups.google.com/forum/#!topic/golang-nuts/pMUkBlQBDF0
	if c.Request.TLS != nil {
		return "https"
	}
	if !trustedProxySource(c.Request.RemoteAddr) {
		return "http"
	}
	if scheme := c.Request.Header.Get("X-Forwarded-Proto"); scheme != "" {
		if scheme == "https" || scheme == "http" {
			return scheme
		}
	}
	if scheme := c.Request.Header.Get("X-Forwarded-Protocol"); scheme != "" {
		if scheme == "https" || scheme == "http" {
			return scheme
		}
	}
	if ssl := c.Request.Header.Get("X-Forwarded-Ssl"); ssl == "on" {
		return "https"
	}
	if scheme := c.Request.Header.Get("X-Url-Scheme"); scheme != "" {
		if scheme == "https" || scheme == "http" {
			return scheme
		}
	}
	return "http"
}

func trustedProxySource(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	for _, value := range strings.Split(os.Getenv("KOMARI_TRUSTED_PROXIES"), ",") {
		value = strings.TrimSpace(value)
		if parsed := net.ParseIP(value); parsed != nil && parsed.Equal(ip) {
			return true
		}
		if _, cidr, err := net.ParseCIDR(value); err == nil && cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func GetCallbackURL(c *gin.Context) string {
	scheme := GetScheme(c)
	host := c.Request.Host
	return scheme + "://" + host + "/api/oauth_callback"
}
