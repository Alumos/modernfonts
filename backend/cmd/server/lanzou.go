package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	defaultLanzouWorkers  = 8
	defaultLanzouCacheTTL = 3 * time.Minute
	maxLanzouFiles        = 200
	maxLanzouFolders      = 20
	maxLanzouPages        = 10
)

type LanzouDownloadFile struct {
	Name  string `json:"name"`
	Size  string `json:"size"`
	Path  string `json:"path,omitempty"`
	URL   string `json:"url,omitempty"`
	Error string `json:"error,omitempty"`
}

type nativeLanzouResolver struct {
	transport *http.Transport
	workers   int
	cacheTTL  time.Duration

	mu       sync.Mutex
	cache    map[string]lanzouCacheEntry
	inFlight map[string]*lanzouResolveCall
}

type lanzouCacheEntry struct {
	files     []LanzouDownloadFile
	expiresAt time.Time
}

type lanzouResolveCall struct {
	done  chan struct{}
	files []LanzouDownloadFile
	err   error
}

type lanzouCandidate struct {
	share string
	name  string
	size  string
	path  string
}

func (rt *Runtime) handleResolveFontDownloads(c *gin.Context) {
	_, db, ok := rt.deps()
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "系统尚未就绪"})
		return
	}
	fontID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || fontID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的字体 ID"})
		return
	}
	var font FontItem
	if err := db.First(&font, uint(fontID)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "字体记录不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取字体记录失败"})
		return
	}
	if !isLanzouShareURL(font.DownloadURL) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该字体不是有效的蓝奏云链接"})
		return
	}
	var site SiteSetting
	if err := db.Order("id").First(&site).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "读取下载设置失败"})
		return
	}

	resolveContext, cancel := context.WithTimeout(c.Request.Context(), 2*time.Minute)
	defer cancel()
	files, err := rt.getLanzouResolver().Resolve(resolveContext, font.DownloadURL, site.LanzouPassword)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	if files == nil {
		files = make([]LanzouDownloadFile, 0)
	}
	c.JSON(http.StatusOK, gin.H{"files": files})
}

func (rt *Runtime) getLanzouResolver() *nativeLanzouResolver {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.lanzouResolver == nil {
		workers := envPositiveInt("LANZOU_WORKERS", defaultLanzouWorkers)
		cacheSeconds := envPositiveInt("LANZOU_CACHE_TTL_SECONDS", int(defaultLanzouCacheTTL/time.Second))
		rt.lanzouResolver = newNativeLanzouResolver(workers, time.Duration(cacheSeconds)*time.Second)
	}
	return rt.lanzouResolver
}

func newNativeLanzouResolver(workers int, cacheTTL time.Duration) *nativeLanzouResolver {
	if workers < 1 {
		workers = defaultLanzouWorkers
	}
	if workers > 16 {
		workers = 16
	}
	if cacheTTL <= 0 {
		cacheTTL = defaultLanzouCacheTTL
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 32
	transport.IdleConnTimeout = 90 * time.Second
	return &nativeLanzouResolver{
		transport: transport,
		workers:   workers,
		cacheTTL:  cacheTTL,
		cache:     make(map[string]lanzouCacheEntry),
		inFlight:  make(map[string]*lanzouResolveCall),
	}
}

func (r *nativeLanzouResolver) Resolve(ctx context.Context, shareURL, password string) ([]LanzouDownloadFile, error) {
	key := lanzouCacheKey(shareURL, password)
	now := time.Now()
	r.mu.Lock()
	if entry, ok := r.cache[key]; ok {
		if now.Before(entry.expiresAt) {
			files := cloneLanzouFiles(entry.files)
			r.mu.Unlock()
			return files, nil
		}
		delete(r.cache, key)
	}
	if call, ok := r.inFlight[key]; ok {
		r.mu.Unlock()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-call.done:
			return cloneLanzouFiles(call.files), call.err
		}
	}
	call := &lanzouResolveCall{done: make(chan struct{})}
	r.inFlight[key] = call
	r.mu.Unlock()

	files, err := r.resolveUncached(ctx, shareURL, password)
	r.mu.Lock()
	call.files, call.err = cloneLanzouFiles(files), err
	if err == nil {
		r.cache[key] = lanzouCacheEntry{files: cloneLanzouFiles(files), expiresAt: time.Now().Add(r.cacheTTL)}
	}
	delete(r.inFlight, key)
	close(call.done)
	r.mu.Unlock()
	return files, err
}

func (r *nativeLanzouResolver) resolveUncached(ctx context.Context, shareURL, password string) ([]LanzouDownloadFile, error) {
	loaded, err := r.loadSharePage(ctx, shareURL)
	if err != nil {
		return nil, err
	}
	if !isLanzouFolderPage(loaded.body) {
		file, err := r.resolveSingleLoaded(ctx, loaded, password, "", "", "")
		if err != nil {
			return nil, err
		}
		return []LanzouDownloadFile{file}, nil
	}

	candidates, err := r.collectFolderCandidates(ctx, loaded, password)
	if err != nil {
		return nil, err
	}
	return r.resolveCandidates(ctx, candidates, password), nil
}

func (r *nativeLanzouResolver) resolveCandidates(ctx context.Context, candidates []lanzouCandidate, password string) []LanzouDownloadFile {
	results := make([]LanzouDownloadFile, len(candidates))
	if len(candidates) == 0 {
		return results
	}
	jobs := make(chan int, len(candidates))
	for index := range candidates {
		jobs <- index
	}
	close(jobs)

	workerCount := r.workers
	if len(candidates) < workerCount {
		workerCount = len(candidates)
	}
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer workers.Done()
			for index := range jobs {
				candidate := candidates[index]
				file, err := r.resolveSingle(ctx, candidate, password)
				if err != nil {
					results[index] = LanzouDownloadFile{Name: candidate.name, Size: candidate.size, Path: candidate.path, Error: err.Error()}
					continue
				}
				results[index] = file
			}
		}()
	}
	workers.Wait()
	return results
}

func (r *nativeLanzouResolver) resolveSingle(ctx context.Context, candidate lanzouCandidate, password string) (LanzouDownloadFile, error) {
	loaded, err := r.loadSharePage(ctx, candidate.share)
	if err != nil {
		return LanzouDownloadFile{}, err
	}
	return r.resolveSingleLoaded(ctx, loaded, password, candidate.name, candidate.size, candidate.path)
}

func envPositiveInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || value < 1 {
		return fallback
	}
	return value
}

func lanzouCacheKey(shareURL, password string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(shareURL) + "\x00" + password))
	return fmt.Sprintf("%x", sum[:])
}

func cloneLanzouFiles(files []LanzouDownloadFile) []LanzouDownloadFile {
	if len(files) == 0 {
		return make([]LanzouDownloadFile, 0)
	}
	return append(make([]LanzouDownloadFile, 0, len(files)), files...)
}

func isLanzouShareURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	return strings.Contains(host, "lanzou") || host == "woozooo.com" || strings.HasSuffix(host, ".woozooo.com")
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
