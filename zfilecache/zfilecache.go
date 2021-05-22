// +build !js

package zfilecache

import (
	"fmt"
	"hash/fnv"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztimer"
)

type Cache struct {
	workDir         string
	cacheName       string
	getURL          string
	ServeEmptyImage bool
	Valid           time.Duration
	UseToken        bool
}

func (c Cache) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	spath := req.URL.Path[1:]
	zstr.HasPrefix(req.URL.Path, zrest.AppURLPrefix, &spath)
	fpath := path.Join(c.workDir, spath)
	dir, file := filepath.Split(fpath)
	file = filepath.Join(dir, file[:3]+"/"+file[3:])
	// zlog.Info("FileCache serve:", file, spath)
	if c.ServeEmptyImage && !zfile.Exists(file) {
		zlog.Info("Serve empty cached image:", file)
		file = "www/images/empty.png"
	}
	http.ServeFile(w, req, file)
}

func Init(workDir, urlPrefix, cacheName string) *Cache {
	if !strings.HasPrefix(urlPrefix, "/") {
		zlog.Error(nil, "url should start with /", urlPrefix)
	}
	if !strings.HasSuffix(urlPrefix, "/") {
		zlog.Fatal(nil, "url should end with /", urlPrefix)
	}
	c := &Cache{}
	c.Valid = time.Hour * 24
	c.workDir = workDir
	c.cacheName = cacheName
	c.getURL = urlPrefix + cacheName
	err := os.MkdirAll(c.workDir+cacheName, 0775|os.ModeDir)
	if err != nil {
		zlog.Log(err, zlog.FatalLevel, "zfilecaches.Init mkdir failed")
	}
	zrest.Handle(c.getURL, c)
	// zlog.Info("Handle:", "/qtt"+c.getURL)
	ztimer.RepeatNow(3600+200*rand.Float64(), func() bool {
		dir := c.workDir + c.cacheName
		// zlog.Info("\n\n##Start cache:", dir)
		zfile.Walk(dir, "*.jpeg\t*.png", func(fpath string, info os.FileInfo) error {
			if c.Valid != 0 && time.Since(info.ModTime()) > c.Valid {
				// zlog.Info("Remove cache:", fpath)
				os.Remove(fpath)
			}
			return nil
		})
		// zlog.Info("\n\n**Finished cache:", cacheName)
		return true
	})
	// zlog.Info("init zfilecache:", c.workDir+c.cacheName, c.getURL)
	return c
}

func (c *Cache) getToken() string {
	return ""
}

func (c *Cache) IsTokenValid(t string) bool {
	return true
}

// CacheFromReader reads bytes from reader with stype png or jpeg and caches is with CacheFromData
// This name is used to get a path or a url for getting
func (c *Cache) CacheFromReader(reader io.Reader, ext string) (string, error) {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", zlog.Error(err, "read from reader")
	}
	return c.CacheFromData(data, ext)
}

// CacheFromData reads in data saving with hash name and extension
// This string return is a hash of data, used to get a path or a url for fetching.
func (c *Cache) CacheFromData(data []byte, ext string) (string, error) {
	var err error
	h := fnv.New64a()
	h.Write(data)

	hash := h.Sum64()
	name := fmt.Sprintf("%x%s", hash, ext)

	path, dir := c.GetPathForName(name)
	err = zfile.MakeDirAllIfNotExists(dir)
	if err != nil {
		return "", zlog.Error(err, "make dir", dir)
	}
	// outUrl := getURLForName(name)

	// zlog.Info("CacheFromReader path:", path)
	if zfile.Exists(path) {
		err = zfile.SetModified(path, time.Now())
		return name, err
	}
	file, err := os.Create(path)
	if err != nil {
		return "", zlog.Error(err, "create file", path)
	}
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return "", zlog.Error(err, "write to file")
	}
	return name, nil
}

func (c *Cache) GetPathForName(name string) (path, dir string) {
	dir = c.workDir + c.cacheName + name[:3] + "/"
	path = dir + name[3:]
	return
}

func (c *Cache) GetUrlForName(name string) string {
	addAppURL := true
	str := c.getURL
	if addAppURL {
		str = path.Join(zrest.AppURLPrefix, str)
	}
	str = path.Join(str, name)
	if c.UseToken {
		str += "?token=" + c.getToken()
	}
	return str
}

func (c *Cache) HandlerWithCheckToken(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t := req.URL.Query().Get("token")
		if !c.IsTokenValid(t) {
			zrest.ReturnError(w, req, "bad token for get cached file", http.StatusBadRequest)
			return
		}
		h.ServeHTTP(w, req)
	})
}

func (c *Cache) RemoveFileWithName(name string) error {
	path, _ := c.GetPathForName(name)
	return os.Remove(path)
}
