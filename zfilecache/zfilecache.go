//go:build !js
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

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztimer"
)

type Cache struct {
	workDir           string
	cacheName         string
	getURL            string
	urlPrefix         string
	ServeEmptyImage   bool
	DeleteAfter       time.Duration // Delete files when modified more than this long ago
	UseToken          bool          // Not implemented
	DeleteRatio       float32       // When deleting files, only do some of sub-folders (randomly) each time 0.1 is do 10% of them at random
	NestInHashFolders bool
}

func (c Cache) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	spath := req.URL.Path[1:]
	zstr.HasPrefix(req.URL.Path, zrest.AppURLPrefix, &spath)
	fpath := path.Join(c.workDir, spath)
	dir, file := filepath.Split(fpath)
	post := file
	if c.NestInHashFolders {
		post = file[:3] + "/" + file[3:]
	}
	file = filepath.Join(dir, post)
	// zlog.Info("FileCache serve:", file, spath)
	if c.ServeEmptyImage && !zfile.Exists(file) {
		zlog.Info("Serve empty cached image:", file)
		file = "www/images/empty.png"
	}
	// zlog.Info("Serve cached image:", req.URL.Path, file)
	http.ServeFile(w, req, file)
}

func Init(router *mux.Router, workDir, urlPrefix, cacheName string) *Cache {
	// if strings.HasPrefix(urlPrefix, "/") {
	// 	zlog.Error(nil, "url should not start with /", urlPrefix)
	// }
	if urlPrefix != "" && !strings.HasSuffix(urlPrefix, "/") {
		zlog.Fatal(nil, "url should end with /", urlPrefix)
	}
	c := &Cache{}
	c.urlPrefix = urlPrefix
	c.DeleteAfter = time.Hour * 24
	c.workDir = workDir
	c.cacheName = cacheName
	c.DeleteRatio = 1
	c.NestInHashFolders = true
	// zlog.Info("zfilecache Init:", workDir, urlPrefix, cacheName)
	path := zstr.Concat("/", urlPrefix, cacheName)
	//	c.getURL = zstr.Concat("/", zrest.AppURLPrefix, path)
	c.getURL = path
	err := os.MkdirAll(c.workDir+cacheName, 0775|os.ModeDir)
	if err != nil {
		zlog.Error(err, zlog.FatalLevel, "zfilecaches.Init mkdir failed")
	}
	zrest.AddSubHandler(router, path, c)
	// zrest.AddHandler(router, strings.TrimRight(path, "/"), c.ServeHTTP)
	ztimer.RepeatNow(1800+200*rand.Float64(), func() bool {
		// start := time.Now()
		dir := c.workDir + c.cacheName
		cutoff := time.Now().Add(-c.DeleteAfter)
		err := zfile.DeleteOldInSubFolders(dir, time.Millisecond*1, cutoff, c.DeleteRatio, func(p float32, count, total int) {
			// zlog.Info("DeleteCache:", dir, int(p*100), count, "/", total)
		})
		if err != nil {
			zlog.Error(err, "delete cache", c.cacheName)
		}
		// zlog.Info("Deleted cache:", c.workDir+c.cacheName, time.Since(start))
		return true
	})
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
func (c *Cache) CacheFromReader(reader io.Reader, name string) (string, error) {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", zlog.Error(err, "read from reader")
	}
	return c.CacheFromData(data, name)
}

// CacheFromData reads in data, using name as filename, or hash of data, and extension
// This string return is this name/hash, used to get a path or a url for fetching.
// This should really call CacheFromReader, not visa versa
func (c *Cache) CacheFromData(data []byte, name string) (string, error) {
	var err error
	if name == "" || strings.HasPrefix(name, ".") {
		h := fnv.New64a()
		h.Write(data)
		hash := h.Sum64()
		name = fmt.Sprintf("%x", hash) + name
	}
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
	lastDir := ""
	end := name
	if c.NestInHashFolders {
		lastDir = "/" + name[:3]
		end = "/" + name[3:]
	}
	dir = filepath.Join(c.workDir, c.urlPrefix, c.cacheName) + lastDir
	path = dir + "/" + end
	// zlog.Info("GetPathForName:", c.workDir, c.urlPrefix, c.cacheName, name, path)
	return
}

func (c *Cache) GetWorkDirectoryStart() string {
	return filepath.Join(c.workDir, c.urlPrefix) + "/"
}

func (c *Cache) IsCached(name string) bool {
	path, _ := c.GetPathForName(name)
	return zfile.Exists(path)
}

func (c *Cache) GetURLForName(name string) string {
	str := zstr.Concat("/", c.getURL, name) // zrest.AppURLPrefix,
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
