package zimagecache

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
	"github.com/torlangballe/zutil/ztimer"
)

type Cache struct {
	workDir   string
	cacheName string
	getURL    string
	Valid     time.Duration
	UseToken  bool
}

func (c Cache) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	spath := req.URL.Path[1:]
	file := path.Join(c.workDir, spath)
	// zlog.Info("FileCache serve:", file)
	if !zfile.Exists(file) {
		zlog.Info("Serve empty cached image:", file)
		file = "www/images/empty.png"
	}
	http.ServeFile(w, req, file)
}

func InitCache(workDir, urlPrefix, cacheName string) *Cache {
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
		zlog.Log(err, zlog.FatalLevel, "zimages.Init mkdir failed")
	}
	http.Handle(c.getURL, c)
	ztimer.RepeatNow(3600+200*rand.Float64(), func() bool {
		dir := c.workDir + c.cacheName
		zfile.Walk(dir, "*.jpeg\t*.png", func(fpath string, info os.FileInfo) error {
			if time.Since(info.ModTime()) > c.Valid {
				os.Remove(fpath)
			}
			return nil
		})
		return true
	})
	// zlog.Info("init image cache:", c.workDir+c.cacheName, c.getURL)
	return c
}

func (c *Cache) getToken() string {
	return ""
}

func (c *Cache) IsTokenValid(t string) bool {
	return true
}

// CacheImageFromReader reads bytes from reader with stype png or jpeg and caches is with CacheImageFromData
// This name is used to get a path or a url for getting
func (c *Cache) CacheImageFromReader(reader io.Reader, stype string) (string, error) {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", zlog.Error(err, "read from reader")
	}
	return c.CacheImageFromData(data, stype)
}

// CacheImageFromData reads a stype png or jpeg image in data saving with name png/jpeg
// This name is a hash of data, used to get a path or a url for getting
func (c *Cache) CacheImageFromData(data []byte, stype string) (string, error) {
	var err error
	h := fnv.New64a()
	h.Write(data)

	hash := h.Sum64()
	name := fmt.Sprintf("%x.%s", hash, stype)

	path := c.GetCachePathForName(name)
	// outUrl := getURLForName(name)

	// zlog.Info("CacheImageFromReader path:", path)
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

func (c *Cache) GetCachePathForName(name string) string {
	return filepath.Join(c.workDir+c.cacheName, name)
}

func (c *Cache) GetCacheUrlForName(name string) string {
	str := c.getURL + name
	if c.UseToken {
		str += "?token=" + c.getToken()
	}
	return str
}

func (c *Cache) CheckCacheToken(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t := req.URL.Query().Get("token")
		if !c.IsTokenValid(t) {
			zrest.ReturnError(w, req, "bad token for get image", http.StatusBadRequest)
			return
		}
		h.ServeHTTP(w, req)
	})
}
