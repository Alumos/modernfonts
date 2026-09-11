package main

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const archiveTTL = 30 * time.Minute

type archiveStore struct {
	root string
	mu   sync.Mutex
}
type archiveEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size uint64 `json:"size"`
}
type archiveSession struct {
	ID    string         `json:"session_id"`
	Name  string         `json:"name"`
	Files []archiveEntry `json:"files"`
}

func newArchiveStore(root string) *archiveStore { return &archiveStore{root: root} }
func (s *archiveStore) cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.root, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return err
	}
	cutoff := time.Now().Add(-archiveTTL)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = os.RemoveAll(filepath.Join(s.root, e.Name()))
		}
	}
	return nil
}
func validArchivePath(p string) bool {
	p = filepath.ToSlash(p)
	return p != "" && !strings.HasPrefix(p, "/") && p != "." && !strings.Contains(p, "../") && !strings.Contains(p, "\\")
}
func (rt *Runtime) handleCreateArchive(c *gin.Context) {
	var req struct {
		URL  string `json:"url"`
		Name string `json:"name"`
	}
	if c.ShouldBindJSON(&req) != nil || !isHTTPURL(req.URL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的压缩包地址"})
		return
	}
	if rt.archiveStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "临时文件服务尚未就绪"})
		return
	}
	idb := make([]byte, 16)
	if _, err := rand.Read(idb); err != nil {
		c.JSON(500, gin.H{"error": "创建临时会话失败"})
		return
	}
	id := hex.EncodeToString(idb)
	dir := filepath.Join(rt.archiveStore.root, id)
	if err := os.MkdirAll(dir, 0700); err != nil {
		c.JSON(500, gin.H{"error": "创建临时目录失败"})
		return
	}
	resp, err := http.Get(req.URL)
	if err != nil {
		_ = os.RemoveAll(dir)
		c.JSON(502, gin.H{"error": "下载压缩包失败"})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = os.RemoveAll(dir)
		c.JSON(502, gin.H{"error": fmt.Sprintf("下载压缩包失败：HTTP %d", resp.StatusCode)})
		return
	}
	archivePath := filepath.Join(dir, "archive.zip")
	f, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		_ = os.RemoveAll(dir)
		c.JSON(500, gin.H{"error": "保存压缩包失败"})
		return
	}
	_, err = io.Copy(f, io.LimitReader(resp.Body, 512<<20))
	f.Close()
	if err != nil {
		_ = os.RemoveAll(dir)
		c.JSON(502, gin.H{"error": "保存压缩包失败"})
		return
	}
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		_ = os.RemoveAll(dir)
		c.JSON(400, gin.H{"error": "文件不是有效的 ZIP 压缩包"})
		return
	}
	defer zr.Close()
	files := make([]archiveEntry, 0, len(zr.File))
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() {
			continue
		}
		if !validArchivePath(zf.Name) {
			_ = os.RemoveAll(dir)
			c.JSON(400, gin.H{"error": "压缩包包含不安全路径"})
			return
		}
		files = append(files, archiveEntry{Name: filepath.Base(zf.Name), Path: filepath.ToSlash(zf.Name), Size: zf.UncompressedSize64})
	}
	if req.Name == "" {
		req.Name = "archive.zip"
	}
	c.JSON(http.StatusOK, archiveSession{ID: id, Name: req.Name, Files: files})
}
func (rt *Runtime) handleArchiveFile(c *gin.Context) {
	id, path := c.Param("id"), c.Query("path")
	if len(id) != 32 || !validArchivePath(path) {
		c.JSON(400, gin.H{"error": "无效的文件路径"})
		return
	}
	archivePath := filepath.Join(rt.archiveStore.root, id, "archive.zip")
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		c.JSON(404, gin.H{"error": "临时文件已过期"})
		return
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if filepath.ToSlash(zf.Name) != filepath.ToSlash(path) {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			break
		}
		defer rc.Close()
		name := filepath.Base(zf.Name)
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="download"; filename*=UTF-8''%s`, url.QueryEscape(name)))
		c.Header("Content-Type", "application/octet-stream")
		_, _ = io.Copy(c.Writer, rc)
		return
	}
	c.JSON(404, gin.H{"error": "文件不存在"})
}

func (rt *Runtime) handleArchiveFont(c *gin.Context) {
	id, path := c.Param("id"), c.Query("path")
	if len(id) != 32 || !validArchivePath(path) || !strings.HasSuffix(strings.ToLower(path), ".ttf") && !strings.HasSuffix(strings.ToLower(path), ".otf") && !strings.HasSuffix(strings.ToLower(path), ".woff") && !strings.HasSuffix(strings.ToLower(path), ".woff2") {
		c.JSON(400, gin.H{"error": "不是可预览的字体文件"})
		return
	}
	zr, err := zip.OpenReader(filepath.Join(rt.archiveStore.root, id, "archive.zip"))
	if err != nil {
		c.JSON(404, gin.H{"error": "临时文件已过期"})
		return
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if filepath.ToSlash(zf.Name) != filepath.ToSlash(path) {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			break
		}
		defer rc.Close()
		c.Header("Content-Type", fontContentType(path))
		c.Header("Cache-Control", "private, max-age=1800")
		_, _ = io.Copy(c.Writer, rc)
		return
	}
	c.JSON(404, gin.H{"error": "文件不存在"})
}
func fontContentType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ttf":
		return "font/ttf"
	case ".otf":
		return "font/otf"
	case ".woff":
		return "font/woff"
	default:
		return "font/woff2"
	}
}
