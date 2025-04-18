//go:build !js

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

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

type Cache struct {
	WorkDir            string
	URL                string
	ServeEmptyImage    bool
	DeleteAfter        time.Duration // Delete files when modified more than this long ago
	UseToken           bool          // Not implemented
	DeleteRatio        float32       // When deleting files, only do some of sub-folders (randomly) each time 0.1 is do 10% of them at random
	NestInHashFolders  bool
	InterceptServeFunc func(w http.ResponseWriter, req *http.Request, file *string) bool // return true if handled. file set on call, can be changed

	urlPrefix  string
	cacheName  string
	lastDelete time.Time
}

func (c Cache) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// zlog.Info("zfilecache.ServeHTTP:", req.Header.Get("Origin"))
	zrest.AddCORSHeaders(w, req)
	spath := req.URL.Path[1:]
	zstr.HasPrefix(req.URL.Path, zrest.AppURLPrefix, &spath)
	fpath := path.Join(c.WorkDir, spath)
	dir, file := filepath.Split(fpath)
	post := file
	if c.NestInHashFolders {
		post = file[:3] + "/" + file[3:]
	}
	file = zfile.JoinPathParts(dir, post)
	if c.InterceptServeFunc != nil && c.InterceptServeFunc(w, req, &file) {
		return
	}
	// zlog.Info("zfilecache serve:", req.URL.String(), zfile.Exists(file))
	if c.ServeEmptyImage && !zfile.Exists(file) {
		zlog.Info("Serve empty cached image:", file)
		file = zrest.StaticFolderPathFunc("/images/empty.png")
	}
	// zlog.Info("Serve cached image:", req.URL.Path, file)
	// zlog.Warn("FileCache serve:", file, spath, zfile.Exists(file), zfile.Size(file))
	http.ServeFile(w, req, file)
}

func (cache *Cache) SetDays(days float64) {
	dur := time.Duration(float64(ztime.Day) * days)
	cache.DeleteAfter = dur
}

func Init(router *mux.Router, workDir, urlPrefix, cacheName string) *Cache {
	// if strings.HasPrefix(urlPrefix, "/") {
	// 	zlog.Error("url should not start with /", urlPrefix)
	// }
	c := &Cache{}
	c.urlPrefix = urlPrefix
	c.DeleteAfter = time.Hour * 24
	c.WorkDir = workDir
	c.cacheName = cacheName
	c.DeleteRatio = 1
	c.NestInHashFolders = true
	path := zstr.Concat("/", urlPrefix, cacheName)
	//	c.URL = zstr.Concat("/", zrest.AppURLPrefix, path)
	c.URL = path
	// err := os.MkdirAll(c.workDir+cacheName, 0775|os.ModeDir)
	// if err != nil {
	// 	zlog.Error(zlog.FatalLevel, "zfilecaches.Init mkdir failed", err)
	// }
	// zlog.Info("zfilecache.AddHandler:", path)
	zrest.AddSubHandler(router, path, c)
	ztimer.RepeatNow(2, func() bool {
		start := time.Now()
		if c.DeleteAfter == 0 {
			return false
		}
		dir := zfile.JoinPathParts(c.WorkDir, c.urlPrefix, c.cacheName)
		err := zfile.MakeDirAllIfNotExists(dir)
		if zlog.OnError(err) {
			return false
		}
		if ztime.Since(c.lastDelete) < 1800+200*rand.Float64() {
			return true
		}
		zlog.Info("DeleteCache:", dir, time.Since(c.lastDelete))
		c.lastDelete = start
		cutoff := time.Now().Add(-c.DeleteAfter)
		err = zfile.DeleteOldInSubFolders(dir, time.Millisecond*1, cutoff, c.DeleteRatio, func(p float32, count, total int) {
			zlog.Info("DeleteCache:", dir, int(p*100), count, "/", total)
		})
		if err != nil {
			zlog.Error("delete old in cache", c.cacheName, err)
		}
		zlog.Info("Deleted in cache:", dir, time.Since(start))
		return true
	})
	// zlog.Info("zfilecache Init:", c.WorkDir+cacheName, c.URL, path)
	return c
}

func (c *Cache) ForceDelete() {
	c.lastDelete = time.Time{}
}

func (c *Cache) getToken() string {
	return ""
}

// CacheFromReader reads bytes from reader with stype png or jpeg and caches is with CacheFromData
// This name is used to get a path or a url for getting
func (c *Cache) CacheFromReader(reader io.Reader, name string) (string, error) {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", zlog.Error("read from reader", err)
	}
	return c.CacheFromData(data, name)
}

// CacheFromData reads in data, using name as filename, or hash of data, and extension
// This string return is this name/hash, used to get a path or a url for fetching.
// This should really call CacheFromReader, not visa versa
func (c *Cache) CacheFromData(data []byte, name string) (string, error) {
	prof := zlog.NewProfile(0.4, "CacheFromData:", name)
	var err error
	// zlog.Warn("CacheFromData1:", name)
	hashName := (name == "" || strings.HasPrefix(name, "."))
	if hashName {
		h := fnv.New64a()
		h.Write(data)
		hash := h.Sum64()
		name = fmt.Sprintf("%x", hash) + name
	}
	path, dir := c.GetPathForName(name)
	prof.Log("After get path")
	// zlog.Warn("CacheFromData:", name, path)
	err = zfile.MakeDirAllIfNotExists(dir)
	if err != nil {
		return "", zlog.Error("make dir", dir, err)
	}
	if hashName && zfile.Exists(path) {
		err = zfile.SetModified(path, time.Now())
		return name, err
	}
	prof.Log("After set mod")
	file, err := os.Create(path)
	if err != nil {
		return "", zlog.Error("create file", path, err)
	}
	prof.Log("After create")
	defer file.Close()

	_, err = file.Write(data)
	if err != nil {
		return "", zlog.Error("write to file", err)
	}
	prof.Log("After Write:", len(data))
	prof.End("")
	return name, nil
}

func (c *Cache) GetPathForName(name string) (path, dir string) {
	dir, name = filepath.Split(name)
	lastDir := ""
	end := name
	if c.NestInHashFolders {
		lastDir = "/" + name[:3]
		end = "/" + name[3:]
	}
	dir = zfile.JoinPathParts(c.WorkDir, c.urlPrefix, c.cacheName, dir) + lastDir
	path = dir + "/" + end
	// zlog.Info("GetPathForName:", c.WorkDir, c.urlPrefix, c.cacheName, name, path)
	return path, dir
}

func (c *Cache) IsCached(name string) bool {
	path, _ := c.GetPathForName(name)
	return zfile.Exists(path)
}

func (c *Cache) URLForName(name string) string {
	str := zstr.Concat("/", c.URL, name) // zrest.AppURLPrefix,
	if c.UseToken {
		str += "?token=" + c.getToken()
	}
	return str
}

func (c *Cache) NameFromURL(surl string) string {
	var name string
	surl = zstr.HeadUntil(surl, "?")
	has := zstr.HasPrefix(surl, c.URL, &name)
	zlog.ErrorIf(!has, surl)
	return zfile.RemovedExtension(name)
}

func (c *Cache) HandlerWithCheckToken(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		h.ServeHTTP(w, req)
	})
}

func (c *Cache) RemoveFileWithName(name string) error {
	path, _ := c.GetPathForName(name)
	return os.Remove(path)
}
