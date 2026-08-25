package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func TestCalculateACWCookie(t *testing.T) {
	body := []byte(`<html><script>var arg1='50263C25DD970F53E6CC69E4B398570185A7C584';document.cookie='acw_sc__v2=';</script></html>`)
	got, ok := calculateACWCookie(body)
	if !ok {
		t.Fatal("challenge was not detected")
	}
	if got != "6a549435dd452998b6dc8796c6106e26c4a85f02" {
		t.Fatalf("cookie = %q", got)
	}
}

func TestNativeLanzouResolverResolvesFolderAndCachesResult(t *testing.T) {
	var folderCalls atomic.Int32
	var filePageCalls atomic.Int32
	var ajaxCalls atomic.Int32
	var headCalls atomic.Int32
	var server *httptest.Server

	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/folder":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<html><head><title>字体文件夹</title></head><body><script>
var pageToken='100'; var keyToken='key'; var pgs; pgs=1;
function file(){ var pwd=document.getElementById('pwd').value; $.ajax({
url:'/filemoreajax.php?file=7', data:{'lx':2,'fid':7,'uid':'9','puid':'puid','pg':pgs,'rep':'0','t':pageToken,'k':keyToken,'up':1,'ls':1,'pwd':pwd}, dataType:'json'}); }
</script></body></html>`)
		case "/filemoreajax.php":
			folderCalls.Add(1)
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse folder form: %v", err)
			}
			if r.Form.Get("pwd") != "test-pass" || r.Form.Get("fid") != "7" || r.Form.Get("t") != "100" {
				t.Errorf("folder form = %#v", r.Form)
			}
			writeJSON(t, w, map[string]any{"zt": 1, "info": "success", "text": []map[string]any{
				{"id": "a", "name_all": "字体 A.zip", "size": "2 M", "t": 0},
				{"id": "b", "name_all": "字体 B.ttf", "size": "800 K", "t": 0},
			}})
		case "/a", "/b":
			filePageCalls.Add(1)
			id := r.URL.Path[1:]
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<html><head><title>字体 %s - 蓝奏云</title><meta name="description" content="文件大小：1 M"></head><body><iframe src="/iframe/%s"></iframe></body></html>`, id, id)
		case "/iframe/a", "/iframe/b":
			id := r.URL.Path[len("/iframe/"):]
			fileID := "11"
			endpoint := "/ajaxm.php"
			if id == "b" {
				fileID = "22"
				endpoint = "/ajaxfile.php"
			}
			fmt.Fprintf(w, `<script>var wp_sign='sign-%s'; var ajaxdata='ajax-%s'; $.ajax({url:'%s?file=%s'});</script>`, id, id, endpoint, fileID)
		case "/ajaxm.php", "/ajaxfile.php":
			ajaxCalls.Add(1)
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse ajax form: %v", err)
			}
			id := "a"
			if r.URL.Query().Get("file") == "22" {
				id = "b"
			}
			if r.Form.Get("sign") != "sign-"+id || r.Form.Get("signs") != "ajax-"+id {
				t.Errorf("ajax form = %#v", r.Form)
			}
			writeJSON(t, w, map[string]any{"zt": 1, "inf": 0, "dom": server.URL, "url": "download/" + id})
		case "/file/download/a", "/file/download/b":
			headCalls.Add(1)
			if r.Header.Get("Referer") != lanzouDownloadReferer || r.Header.Get("X-Forwarded-For") != "0.0.0.0" {
				t.Errorf("download headers = %#v", r.Header)
			}
			if cookie, err := r.Cookie("down_ip"); err != nil || cookie.Value != "1" {
				t.Errorf("down_ip cookie = %#v, %v", cookie, err)
			}
			w.Header().Set("Location", "/cdn/"+r.URL.Path[len("/file/download/"):])
			w.WriteHeader(http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	resolver := newNativeLanzouResolver(2, time.Minute)
	files, err := resolver.Resolve(t.Context(), server.URL+"/folder", "test-pass")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %#v", files)
	}
	if files[0].Name != "字体 A.zip" || files[0].Size != "2 M" || files[0].URL != server.URL+"/cdn/a" {
		t.Fatalf("first file = %#v", files[0])
	}
	if files[1].URL != server.URL+"/cdn/b" {
		t.Fatalf("second file = %#v", files[1])
	}
	if folderCalls.Load() != 1 || filePageCalls.Load() != 2 || ajaxCalls.Load() != 2 || headCalls.Load() != 2 {
		t.Fatalf("calls = folder:%d page:%d ajax:%d head:%d", folderCalls.Load(), filePageCalls.Load(), ajaxCalls.Load(), headCalls.Load())
	}

	if _, err := resolver.Resolve(t.Context(), server.URL+"/folder", "test-pass"); err != nil {
		t.Fatalf("cached resolve: %v", err)
	}
	if folderCalls.Load() != 1 || filePageCalls.Load() != 2 || ajaxCalls.Load() != 2 || headCalls.Load() != 2 {
		t.Fatal("cached resolve made additional requests")
	}
}

func TestCloneLanzouFilesPreservesEmptySlice(t *testing.T) {
	cloned := cloneLanzouFiles([]LanzouDownloadFile{})
	if cloned == nil || len(cloned) != 0 {
		t.Fatalf("cloned files = %#v, want non-nil empty slice", cloned)
	}
}

func TestNativeLanzouResolverSupportsPasswordProtectedFile(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/protected":
			fmt.Fprint(w, `<html><head><title>加密字体.zip - 蓝奏云</title><meta name="description" content="文件大小：3 M"></head><script>function down_p(){}; var data={'sign':'secret-sign'}; var api={url:'/ajaxm.php?file=33'};</script></html>`)
		case "/ajaxm.php":
			if err := r.ParseForm(); err != nil {
				t.Errorf("parse form: %v", err)
			}
			if r.Form.Get("p") != "test-pass" || r.Form.Get("sign") != "secret-sign" {
				t.Errorf("password form = %#v", r.Form)
			}
			writeJSON(t, w, map[string]any{"zt": 1, "inf": "加密字体.zip", "dom": server.URL, "url": "protected-file"})
		case "/file/protected-file":
			w.Header().Set("Location", "/cdn/protected")
			w.WriteHeader(http.StatusFound)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	resolver := newNativeLanzouResolver(1, time.Minute)
	files, err := resolver.Resolve(t.Context(), server.URL+"/protected", "test-pass")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(files) != 1 || files[0].Name != "加密字体.zip" || files[0].Size != "3 M" || files[0].URL != server.URL+"/cdn/protected" {
		t.Fatalf("files = %#v", files)
	}
}

func TestResolveFinalURLDoesNotLeakShareCookies(t *testing.T) {
	var receivedCookie string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCookie = r.Header.Get("Cookie")
		w.Header().Set("Location", "/cdn/font.zip")
		w.WriteHeader(http.StatusFound)
	}))
	t.Cleanup(target.Close)

	resolver := newNativeLanzouResolver(1, time.Minute)
	session, err := resolver.newSession()
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	shareURL, err := url.Parse("https://share.lanzouv.com/font")
	if err != nil {
		t.Fatalf("parse share URL: %v", err)
	}
	session.client.Jar.SetCookies(shareURL, []*http.Cookie{{Name: "share_token", Value: "secret", Path: "/"}})

	got := session.resolveFinalURL(t.Context(), target.URL+"/file/font.zip")
	if got != target.URL+"/cdn/font.zip" {
		t.Fatalf("direct URL = %q", got)
	}
	if receivedCookie != "down_ip=1" {
		t.Fatalf("download cookie = %q", receivedCookie)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Errorf("encode response: %v", err)
	}
}
