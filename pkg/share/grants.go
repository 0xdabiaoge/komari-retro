// Package share owns read-only capabilities. Only token hashes are persisted
// for newly issued grants; existing links remain compatible and revocable.
package share

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"github.com/komari-monitor/komari/pkg/config"
	"net/http"
	"strings"
	"sync"
	"time"
)

const RegistryKey = "viewer_grants"

type Grant struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Hash           string   `json:"token_hash,omitempty"`
	Nodes          []string `json:"nodes"`
	AllNodes       bool     `json:"all_nodes"`
	ShowPrice      bool     `json:"show_price"`
	ShowExpiry     bool     `json:"show_expiry"`
	HistoryHours   int      `json:"history_hours"`
	MaxConnections int      `json:"max_connections"`
	ExpiresAt      int64    `json:"expires_at"`
	CreatedAt      int64    `json:"created_at"`
	Revoked        bool     `json:"revoked"`
}

var registryMu sync.Mutex

func digest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func equal(a, b string) bool {
	return a != "" && b != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
func List() ([]Grant, error) {
	grants, err := config.GetAs[[]Grant](RegistryKey)
	if err != nil { // Missing registry is normal for an upgraded installation.
		if _, e := config.Get(RegistryKey); e != nil {
			return []Grant{}, nil
		}
		return nil, err
	}
	return grants, nil
}
func Create(g Grant) (Grant, string, error) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if len(g.Name) > 100 || len(g.Nodes) > 1000 || (!g.AllNodes && len(g.Nodes) == 0) || g.HistoryHours < 0 || g.HistoryHours > 744 || g.MaxConnections < 1 || g.MaxConnections > 32 || (g.ExpiresAt != 0 && g.ExpiresAt <= time.Now().Unix()) {
		return Grant{}, "", errors.New("Invalid share scope or expiry")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Grant{}, "", err
	}
	token := hex.EncodeToString(raw)
	g.Hash = digest(token)
	g.ID = g.Hash[:24]
	g.CreatedAt = time.Now().Unix()
	g.Revoked = false
	grants, err := List()
	if err != nil {
		return Grant{}, "", err
	}
	active := 0
	for _, old := range grants {
		if !old.Revoked && (old.ExpiresAt == 0 || old.ExpiresAt > time.Now().Unix()) {
			active++
		}
	}
	if active >= 100 {
		return Grant{}, "", errors.New("At most 100 active shares")
	}
	// Bound the registry across repeated creation/revocation cycles.
	kept := make([]Grant, 0, len(grants)+1)
	for _, old := range grants {
		if !old.Revoked && (old.ExpiresAt == 0 || old.ExpiresAt > time.Now().Unix()) {
			kept = append(kept, old)
		}
	}
	kept = append(kept, g)
	if err = config.Set(RegistryKey, kept); err != nil {
		return Grant{}, "", err
	}
	g.Hash = ""
	return g, token, nil
}
func Revoke(id string) error {
	registryMu.Lock()
	defer registryMu.Unlock()
	grants, err := List()
	if err != nil {
		return err
	}
	for i := range grants {
		if grants[i].ID == id {
			grants[i].Revoked = true
			return config.Set(RegistryKey, grants)
		}
	}
	return errors.New("Share not found")
}
func Validate(token string) *Grant {
	if token == "" || len(token) > 256 {
		return nil
	}
	now := time.Now().Unix()
	grants, err := List()
	if err != nil {
		return nil
	}
	hash := digest(token)
	for _, g := range grants {
		if !g.Revoked && (g.ExpiresAt == 0 || g.ExpiresAt > now) && equal(hash, g.Hash) {
			g.Hash = ""
			return &g
		}
	}
	perm, _ := config.GetAs[string](config.PermanentShareTokenKey)
	if equal(token, perm) {
		return &Grant{ID: "legacy-permanent", AllNodes: true, ShowPrice: true, ShowExpiry: true, HistoryHours: 24, MaxConnections: 8}
	}
	temp, _ := config.GetAs[string](config.TemporyShareTokenKey)
	expires, _ := config.GetAs[int64](config.TemporyShareTokenExpireAtKey)
	if expires > now && equal(token, temp) {
		return &Grant{ID: "legacy-temporary", AllNodes: true, ShowPrice: true, ShowExpiry: true, HistoryHours: 24, MaxConnections: 8, ExpiresAt: expires}
	}
	return nil
}
func Token(r *http.Request) string {
	if cookie, err := r.Cookie("temp_key"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	if token := r.URL.Query().Get("temp_key"); token != "" {
		return token
	}
	return r.Header.Get("X-Temp-Key")
}
func FromRequest(r *http.Request) *Grant { return Validate(Token(r)) }
func (g *Grant) Allows(uuid string) bool {
	if g == nil || g.AllNodes {
		return true
	}
	for _, id := range g.Nodes {
		if uuid == id {
			return true
		}
	}
	return false
}

// SQLPredicate returns a code-owned clause and bound arguments for joined client queries.
func (g *Grant) SQLPredicate(column string) (string, []any) {
	if g == nil || g.AllNodes {
		return "", nil
	}
	if len(g.Nodes) == 0 {
		return " AND 1=0", nil
	}
	marks := make([]string, len(g.Nodes))
	args := make([]any, len(g.Nodes))
	for i, id := range g.Nodes {
		marks[i] = "?"
		args[i] = id
	}
	return " AND " + column + " IN (" + strings.Join(marks, ",") + ")", args
}
