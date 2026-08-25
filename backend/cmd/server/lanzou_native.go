package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	lanzouUserAgent       = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
	lanzouDownloadReferer = "https://www.lanzouf.com/"
)

var (
	acwArgRE             = regexp.MustCompile(`arg1=['"]([0-9A-Fa-f]+)['"]`)
	folderEndpointRE     = regexp.MustCompile(`(?is)url\s*:\s*['"]([^'"]*filemoreajax\.php[^'"]*)['"]`)
	folderDataBlockRE    = regexp.MustCompile(`(?is)data\s*:\s*\{(.*?)\}\s*,\s*dataType`)
	jsStringVariableRE   = regexp.MustCompile(`(?m)(?:var\s+)?([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*['"]([^'"]*)['"]\s*;`)
	jsNumberVariableRE   = regexp.MustCompile(`(?m)(?:var\s+)?([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*(\d+)\s*;`)
	jsObjectFieldRE      = regexp.MustCompile(`(?m)['"]([A-Za-z0-9_]+)['"]\s*:\s*([^,\r\n}]+)`)
	subfolderRE          = regexp.MustCompile(`(?is)<div[^>]*class=['"][^'"]*(?:pc-folderlink|mbx\s+mbxfolder)[^'"]*['"][^>]*>.*?<a[^>]*href=['"]/?([^'"?#]+)[^'"]*['"][^>]*>(.*?)</a>`)
	htmlTagRE            = regexp.MustCompile(`(?s)<[^>]+>`)
	titleRE              = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	styledFileNameRE     = regexp.MustCompile(`(?is)<div[^>]*style=['"][^'"]*font-size[^'"]*['"][^>]*>(.*?)</div>`)
	classFileNameRE      = regexp.MustCompile(`(?is)<div[^>]*class=['"][^'"]*n_box_3fn[^'"]*['"][^>]*>(.*?)</div>`)
	metaDescriptionRE    = regexp.MustCompile(`(?is)<meta[^>]*name=['"]description['"][^>]*content=['"]([^'"]*)['"]`)
	fileSizeRE           = regexp.MustCompile(`文件大小[：:]\s*([^|<]+)`)
	iframeRE             = regexp.MustCompile(`(?is)<iframe[^>]*src=['"]([^'"]+)['"]`)
	passwordSignRE       = regexp.MustCompile(`['"]sign['"]\s*:\s*['"]([^'"]+)['"]`)
	ajaxFileIDRE         = regexp.MustCompile(`/ajax(?:m|file)\.php\?file=(\d+)`)
	iframeSignRE         = regexp.MustCompile(`(?:var\s+)?wp_sign\s*=\s*['"]([^'"]+)['"]`)
	iframeAjaxDataRE     = regexp.MustCompile(`(?:var\s+)?ajaxdata\s*=\s*['"]([^'"]+)['"]`)
	downloadVerifyFileRE = regexp.MustCompile(`['"]file['"]\s*:\s*['"]([^'"]+)['"]`)
	downloadVerifySignRE = regexp.MustCompile(`['"]sign['"]\s*:\s*['"]([^'"]+)['"]`)
)

type lanzouSession struct {
	client *http.Client
}

type lanzouHTTPResult struct {
	body     []byte
	finalURL *url.URL
	header   http.Header
	status   int
}

type loadedLanzouPage struct {
	session *lanzouSession
	url     *url.URL
	body    []byte
}

type lanzouFolderJob struct {
	share  string
	path   string
	loaded *loadedLanzouPage
}

type lanzouFolderConfig struct {
	endpoint *url.URL
	values   url.Values
}

type lanzouFolderResponse struct {
	ZT   int             `json:"zt"`
	Info json.RawMessage `json:"info"`
	Text json.RawMessage `json:"text"`
}

type lanzouFolderFile struct {
	ID   string `json:"id"`
	Name string `json:"name_all"`
	Size string `json:"size"`
	Ad   int    `json:"t"`
}

type lanzouSubfolder struct {
	ID   string
	Name string
}

type lanzouAjaxResponse struct {
	ZT  int             `json:"zt"`
	Inf json.RawMessage `json:"inf"`
	Dom string          `json:"dom"`
	URL string          `json:"url"`
}

type lanzouDownloadVerifyResponse struct {
	ZT  int    `json:"zt"`
	URL string `json:"url"`
}

func (r *nativeLanzouResolver) newSession() (*lanzouSession, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &lanzouSession{client: &http.Client{Transport: r.transport, Jar: jar, Timeout: 12 * time.Second}}, nil
}

func (r *nativeLanzouResolver) loadSharePage(ctx context.Context, rawURL string) (*loadedLanzouPage, error) {
	var lastErr error
	for _, candidate := range lanzouShareCandidates(rawURL) {
		session, err := r.newSession()
		if err != nil {
			return nil, err
		}
		result, err := session.request(ctx, http.MethodGet, candidate, nil, "")
		if err != nil {
			lastErr = err
			continue
		}
		if len(result.body) == 0 {
			lastErr = errors.New("蓝奏云页面内容为空")
			continue
		}
		if strings.Contains(string(result.body), "文件取消分享了") {
			return nil, errors.New("蓝奏云文件已取消分享")
		}
		return &loadedLanzouPage{session: session, url: result.finalURL, body: result.body}, nil
	}
	if lastErr == nil {
		lastErr = errors.New("无法访问蓝奏云分享页")
	}
	return nil, lastErr
}

func (s *lanzouSession) request(ctx context.Context, method, rawURL string, form url.Values, referer string) (*lanzouHTTPResult, error) {
	for attempt := 0; attempt < 2; attempt++ {
		var body io.Reader
		if form != nil {
			body = strings.NewReader(form.Encode())
		}
		req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", lanzouUserAgent)
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
		if referer != "" {
			req.Header.Set("Referer", referer)
		}
		if form != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("X-Requested-With", "XMLHttpRequest")
			if parsed, parseErr := url.Parse(rawURL); parseErr == nil {
				req.Header.Set("Origin", parsed.Scheme+"://"+parsed.Host)
			}
		}

		response, err := s.client.Do(req)
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		response.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		finalURL := response.Request.URL
		if token, ok := calculateACWCookie(data); ok && attempt == 0 {
			s.client.Jar.SetCookies(finalURL, []*http.Cookie{{Name: "acw_sc__v2", Value: token, Path: "/"}})
			continue
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, fmt.Errorf("蓝奏云返回 HTTP %d", response.StatusCode)
		}
		return &lanzouHTTPResult{body: data, finalURL: finalURL, header: response.Header.Clone(), status: response.StatusCode}, nil
	}
	return nil, errors.New("蓝奏云反爬验证失败")
}

func (r *nativeLanzouResolver) collectFolderCandidates(ctx context.Context, first *loadedLanzouPage, password string) ([]lanzouCandidate, error) {
	queue := []lanzouFolderJob{{share: first.url.String(), loaded: first}}
	seenFolders := map[string]bool{first.url.String(): true}
	seenFiles := make(map[string]bool)
	var candidates []lanzouCandidate

	for len(queue) > 0 {
		job := queue[0]
		queue = queue[1:]
		loaded := job.loaded
		if loaded == nil {
			var err error
			loaded, err = r.loadSharePage(ctx, job.share)
			if err != nil {
				return nil, err
			}
		}
		if !isLanzouFolderPage(loaded.body) {
			if len(candidates) >= maxLanzouFiles {
				return nil, fmt.Errorf("蓝奏云文件数量超过上限 %d", maxLanzouFiles)
			}
			candidates = append(candidates, lanzouCandidate{share: loaded.url.String(), path: job.path})
			continue
		}

		config, err := parseLanzouFolderConfig(loaded.body, loaded.url)
		if err != nil {
			return nil, err
		}
		for _, folder := range parseLanzouSubfolders(loaded.body) {
			folderURL := resolveLanzouShareID(loaded.url, folder.ID)
			if folderURL == "" || seenFolders[folderURL] {
				continue
			}
			if len(seenFolders) >= maxLanzouFolders {
				return nil, fmt.Errorf("蓝奏云文件夹数量超过上限 %d", maxLanzouFolders)
			}
			seenFolders[folderURL] = true
			queue = append(queue, lanzouFolderJob{share: folderURL, path: joinLanzouPath(job.path, folder.Name)})
		}

		for page := 1; page <= maxLanzouPages; page++ {
			files, haveMore, err := loaded.session.loadFolderPage(ctx, config, loaded.url.String(), password, page)
			if err != nil {
				return nil, err
			}
			for _, file := range files {
				if file.ID == "" || file.ID == "-1" || file.Ad == 1 || seenFiles[file.ID] {
					continue
				}
				if len(candidates) >= maxLanzouFiles {
					return nil, fmt.Errorf("蓝奏云文件数量超过上限 %d", maxLanzouFiles)
				}
				seenFiles[file.ID] = true
				candidates = append(candidates, lanzouCandidate{
					share: resolveLanzouShareID(loaded.url, file.ID),
					name:  stdhtml.UnescapeString(strings.TrimSpace(file.Name)),
					size:  strings.TrimSpace(file.Size),
					path:  job.path,
				})
			}
			if !haveMore {
				break
			}
			if page == maxLanzouPages {
				return nil, fmt.Errorf("蓝奏云文件夹分页超过上限 %d", maxLanzouPages)
			}
		}
	}
	return candidates, nil
}

func parseLanzouFolderConfig(body []byte, pageURL *url.URL) (*lanzouFolderConfig, error) {
	html := string(body)
	endpointMatch := folderEndpointRE.FindStringSubmatch(html)
	blockMatch := folderDataBlockRE.FindStringSubmatch(html)
	if len(endpointMatch) != 2 || len(blockMatch) != 2 {
		return nil, errors.New("无法读取蓝奏云文件夹参数")
	}
	endpointRef, err := url.Parse(stdhtml.UnescapeString(strings.TrimSpace(endpointMatch[1])))
	if err != nil {
		return nil, errors.New("蓝奏云文件夹接口无效")
	}
	variables := make(map[string]string)
	for _, match := range jsStringVariableRE.FindAllStringSubmatch(html, -1) {
		variables[match[1]] = match[2]
	}
	for _, match := range jsNumberVariableRE.FindAllStringSubmatch(html, -1) {
		variables[match[1]] = match[2]
	}
	values := make(url.Values)
	for _, match := range jsObjectFieldRE.FindAllStringSubmatch(blockMatch[1], -1) {
		value := resolveJSExpression(match[2], variables)
		if value != "" {
			values.Set(match[1], value)
		}
	}
	if values.Get("fid") == "" {
		return nil, errors.New("蓝奏云文件夹标识缺失")
	}
	return &lanzouFolderConfig{endpoint: pageURL.ResolveReference(endpointRef), values: values}, nil
}

func (s *lanzouSession) loadFolderPage(ctx context.Context, config *lanzouFolderConfig, referer, password string, page int) ([]lanzouFolderFile, bool, error) {
	values := cloneURLValues(config.values)
	values.Set("pg", strconv.Itoa(page))
	values.Set("pwd", password)
	result, err := s.request(ctx, http.MethodPost, config.endpoint.String(), values, referer)
	if err != nil {
		return nil, false, err
	}
	var response lanzouFolderResponse
	if err := json.Unmarshal(result.body, &response); err != nil {
		return nil, false, errors.New("蓝奏云文件夹返回无效数据")
	}
	switch response.ZT {
	case 1:
		var files []lanzouFolderFile
		if err := json.Unmarshal(response.Text, &files); err != nil {
			return nil, false, errors.New("无法读取蓝奏云文件列表")
		}
		return files, len(files) >= 50, nil
	case 2:
		return []lanzouFolderFile{}, false, nil
	case 3:
		return nil, false, errors.New("蓝奏云密码错误")
	case 4:
		return nil, false, errors.New("蓝奏云文件夹参数已失效，请重试")
	default:
		return nil, false, fmt.Errorf("蓝奏云文件夹解析失败：%s", jsonMessage(response.Info))
	}
}

func (r *nativeLanzouResolver) resolveSingleLoaded(ctx context.Context, loaded *loadedLanzouPage, password, listedName, listedSize, path string) (LanzouDownloadFile, error) {
	body := string(loaded.body)
	name := strings.TrimSpace(listedName)
	if name == "" {
		name = parseLanzouFileName(body)
	}
	size := strings.TrimSpace(listedSize)
	if size == "" {
		size = parseLanzouFileSize(body)
	}

	var ajaxResult *lanzouAjaxResponse
	var err error
	if strings.Contains(body, "function down_p()") || strings.Contains(body, "document.getElementById('pwd').value") {
		if password == "" {
			return LanzouDownloadFile{}, errors.New("蓝奏云链接需要访问密码")
		}
		sign := firstRegexpGroup(passwordSignRE, body)
		fileID := firstRegexpGroup(ajaxFileIDRE, body)
		if sign == "" || fileID == "" {
			return LanzouDownloadFile{}, errors.New("无法读取蓝奏云密码页参数")
		}
		ajaxResult, err = loaded.session.postAjax(ctx, loaded.url, loaded.url.String(), "/ajaxm.php", fileID, url.Values{
			"action": {"downprocess"},
			"sign":   {sign},
			"p":      {password},
			"kd":     {"1"},
		})
	} else {
		iframeSrc := firstRegexpGroup(iframeRE, body)
		if iframeSrc == "" {
			return LanzouDownloadFile{}, errors.New("无法读取蓝奏云下载页面")
		}
		iframeRef, parseErr := url.Parse(stdhtml.UnescapeString(iframeSrc))
		if parseErr != nil {
			return LanzouDownloadFile{}, errors.New("蓝奏云下载页面地址无效")
		}
		iframeURL := loaded.url.ResolveReference(iframeRef)
		iframe, requestErr := loaded.session.request(ctx, http.MethodGet, iframeURL.String(), nil, loaded.url.String())
		if requestErr != nil {
			return LanzouDownloadFile{}, requestErr
		}
		iframeBody := string(iframe.body)
		sign := firstRegexpGroup(iframeSignRE, iframeBody)
		ajaxData := firstRegexpGroup(iframeAjaxDataRE, iframeBody)
		fileID := firstRegexpGroup(ajaxFileIDRE, iframeBody)
		if sign == "" || ajaxData == "" || fileID == "" {
			return LanzouDownloadFile{}, errors.New("无法读取蓝奏云直链参数")
		}
		ajaxResult, err = loaded.session.postAjax(ctx, loaded.url, iframeURL.String(), "/ajaxfile.php", fileID, url.Values{
			"action":     {"downprocess"},
			"websignkey": {ajaxData},
			"signs":      {ajaxData},
			"sign":       {sign},
			"websign":    {""},
			"kd":         {"1"},
			"ves":        {"1"},
		})
	}
	if err != nil {
		return LanzouDownloadFile{}, err
	}
	if ajaxResult.ZT != 1 || ajaxResult.Dom == "" || ajaxResult.URL == "" {
		message := jsonMessage(ajaxResult.Inf)
		if message == "" {
			message = "获取直链失败"
		}
		return LanzouDownloadFile{}, errors.New(message)
	}
	intermediate := strings.TrimRight(ajaxResult.Dom, "/") + "/file/" + strings.TrimLeft(ajaxResult.URL, "/")
	directURL := loaded.session.resolveFinalURL(ctx, intermediate)
	if !isHTTPURL(directURL) {
		return LanzouDownloadFile{}, errors.New("未获取到有效的文件直链")
	}
	return LanzouDownloadFile{Name: name, Size: size, Path: path, URL: directURL}, nil
}

func (s *lanzouSession) postAjax(ctx context.Context, pageURL *url.URL, referer, endpointPath, fileID string, values url.Values) (*lanzouAjaxResponse, error) {
	endpoint := &url.URL{Scheme: pageURL.Scheme, Host: pageURL.Host, Path: endpointPath, RawQuery: "file=" + url.QueryEscape(fileID)}
	result, err := s.request(ctx, http.MethodPost, endpoint.String(), values, referer)
	if err != nil {
		return nil, err
	}
	var response lanzouAjaxResponse
	if err := json.Unmarshal(result.body, &response); err != nil {
		return nil, errors.New("蓝奏云直链接口返回无效数据")
	}
	return &response, nil
}

func (s *lanzouSession) resolveFinalURL(ctx context.Context, rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	s.client.Jar.SetCookies(parsed, []*http.Cookie{{Name: "down_ip", Value: "1", Path: "/"}})
	result, err := s.requestWithoutRedirect(ctx, http.MethodGet, parsed.String(), nil, lanzouDownloadReferer)
	if err == nil {
		if location := strings.TrimSpace(result.header.Get("Location")); location != "" {
			if reference, parseErr := url.Parse(location); parseErr == nil {
				return parsed.ResolveReference(reference).String()
			}
		}
		body := string(result.body)
		file := firstRegexpGroup(downloadVerifyFileRE, body)
		sign := firstRegexpGroup(downloadVerifySignRE, body)
		if file != "" && sign != "" {
			verifyRef, _ := url.Parse("ajax.php")
			verifyURL := parsed.ResolveReference(verifyRef)
			verifyResult, verifyErr := s.request(ctx, http.MethodPost, verifyURL.String(), url.Values{
				"file": {file},
				"el":   {"2"},
				"sign": {sign},
			}, parsed.String())
			if verifyErr == nil {
				var response lanzouDownloadVerifyResponse
				if json.Unmarshal(verifyResult.body, &response) == nil && response.ZT == 1 && isHTTPURL(response.URL) {
					return response.URL
				}
			}
		}
	}
	return s.resolveFinalURLWithHead(ctx, parsed, lanzouDownloadReferer)
}

func (s *lanzouSession) requestWithoutRedirect(ctx context.Context, method, rawURL string, form url.Values, referer string) (*lanzouHTTPResult, error) {
	client := &http.Client{
		Transport: s.client.Transport,
		Jar:       s.client.Jar,
		Timeout:   10 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", lanzouUserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "zh-CN;q=0.9,zh-HK;q=0.8,zh-TW;q=0.7")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("X-Forwarded-For", "0.0.0.0")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	response.Body.Close()
	if readErr != nil {
		return nil, readErr
	}
	if response.StatusCode >= 400 {
		return nil, fmt.Errorf("下载地址返回 HTTP %d", response.StatusCode)
	}
	return &lanzouHTTPResult{body: data, finalURL: response.Request.URL, header: response.Header.Clone(), status: response.StatusCode}, nil
}

func (s *lanzouSession) resolveFinalURLWithHead(ctx context.Context, parsed *url.URL, referer string) string {
	result, err := s.requestWithoutRedirect(ctx, http.MethodHead, parsed.String(), nil, referer)
	if err != nil {
		return parsed.String()
	}
	location := strings.TrimSpace(result.header.Get("Location"))
	if location == "" {
		return parsed.String()
	}
	ref, err := url.Parse(location)
	if err != nil {
		return parsed.String()
	}
	return parsed.ResolveReference(ref).String()
}

func isLanzouFolderPage(body []byte) bool {
	return folderEndpointRE.Match(body)
}

func lanzouShareCandidates(rawURL string) []string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return []string{rawURL}
	}
	result := []string{parsed.String()}
	if !strings.Contains(strings.ToLower(parsed.Hostname()), "lanzou") {
		return result
	}
	seen := map[string]bool{parsed.String(): true}
	for _, host := range []string{"www.lanzouf.com", "www.lanzoux.com", "www.lanzouj.com", "www.lanzouu.com", "www.lanzouw.com"} {
		candidate := *parsed
		candidate.Scheme = "https"
		candidate.Host = host
		value := candidate.String()
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func resolveLanzouShareID(base *url.URL, id string) string {
	id = strings.TrimSpace(strings.TrimPrefix(id, "/"))
	if id == "" {
		return ""
	}
	return (&url.URL{Scheme: base.Scheme, Host: base.Host, Path: "/" + id}).String()
}

func parseLanzouSubfolders(body []byte) []lanzouSubfolder {
	var folders []lanzouSubfolder
	for _, match := range subfolderRE.FindAllSubmatch(body, -1) {
		if len(match) != 3 {
			continue
		}
		folders = append(folders, lanzouSubfolder{ID: string(match[1]), Name: cleanHTMLText(string(match[2]))})
	}
	return folders
}

func resolveJSExpression(expression string, variables map[string]string) string {
	value := strings.TrimSpace(expression)
	if len(value) >= 2 && ((value[0] == '\'' && value[len(value)-1] == '\'') || (value[0] == '"' && value[len(value)-1] == '"')) {
		return value[1 : len(value)-1]
	}
	if resolved, ok := variables[value]; ok {
		return resolved
	}
	if _, err := strconv.Atoi(value); err == nil {
		return value
	}
	return ""
}

func parseLanzouFileName(body string) string {
	for _, expression := range []*regexp.Regexp{classFileNameRE, styledFileNameRE, titleRE} {
		if value := firstRegexpGroup(expression, body); value != "" {
			value = cleanHTMLText(value)
			value = strings.TrimSuffix(value, " - 蓝奏云")
			value = strings.TrimSuffix(value, " 蓝奏云")
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func parseLanzouFileSize(body string) string {
	description := firstRegexpGroup(metaDescriptionRE, body)
	return strings.TrimSpace(firstRegexpGroup(fileSizeRE, stdhtml.UnescapeString(description)))
}

func cleanHTMLText(value string) string {
	return strings.TrimSpace(stdhtml.UnescapeString(htmlTagRE.ReplaceAllString(value, "")))
}

func firstRegexpGroup(expression *regexp.Regexp, value string) string {
	match := expression.FindStringSubmatch(value)
	if len(match) != 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}

func cloneURLValues(values url.Values) url.Values {
	cloned := make(url.Values, len(values))
	for key, items := range values {
		cloned[key] = append([]string(nil), items...)
	}
	return cloned
}

func joinLanzouPath(base, name string) string {
	name = strings.TrimSpace(name)
	if base == "" {
		return name
	}
	if name == "" {
		return base
	}
	return base + " / " + name
}

func jsonMessage(raw json.RawMessage) string {
	var value string
	if err := json.Unmarshal(raw, &value); err == nil {
		return strings.TrimSpace(value)
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err == nil {
		if number.String() == "0" {
			return ""
		}
		return number.String()
	}
	return ""
}

func calculateACWCookie(body []byte) (string, bool) {
	match := acwArgRE.FindSubmatch(body)
	if len(match) != 2 {
		return "", false
	}
	arg := string(match[1])
	positions := []int{15, 35, 29, 24, 33, 16, 1, 38, 10, 9, 19, 31, 40, 27, 22, 23, 25, 13, 6, 11, 39, 18, 20, 8, 14, 21, 32, 26, 2, 30, 7, 4, 17, 5, 3, 28, 34, 37, 12, 36}
	mask := "3000176000856006061501533003690027800375"
	if len(arg) < len(positions) || len(mask) < len(positions) {
		return "", false
	}
	reordered := make([]byte, len(positions))
	for sourceIndex, char := range []byte(arg) {
		for targetIndex, position := range positions {
			if position == sourceIndex+1 {
				reordered[targetIndex] = char
				break
			}
		}
	}
	var result strings.Builder
	for index := 0; index+1 < len(reordered); index += 2 {
		value, valueErr := strconv.ParseUint(string(reordered[index:index+2]), 16, 8)
		maskValue, maskErr := strconv.ParseUint(mask[index:index+2], 16, 8)
		if valueErr != nil || maskErr != nil {
			return "", false
		}
		fmt.Fprintf(&result, "%02x", byte(value)^byte(maskValue))
	}
	return result.String(), true
}
