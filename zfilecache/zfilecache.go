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
	DeleteAfter     time.Duration // Delete files when modified more than this long ago
	UseToken        bool          // Not implemented
	DeleteRatio     float32       // When deleting files, only do some of sub-folders (randomly) each time 0.1 is do 10% of them at random
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
	// zlog.Info("Serve cached image:", req.URL.Path, file)
	http.ServeFile(w, req, file)
}

func Init(workDir, urlPrefix, cacheName string) *Cache {
	// if strings.HasPrefix(urlPrefix, "/") {
	// 	zlog.Error(nil, "url should not start with /", urlPrefix)
	// }
	if urlPrefix != "" && !strings.HasSuffix(urlPrefix, "/") {
		zlog.Fatal(nil, "url should end with /", urlPrefix)
	}
	c := &Cache{}
	c.DeleteAfter = time.Hour * 24
	c.workDir = workDir
	c.cacheName = cacheName
	path := zstr.Concat("/", urlPrefix, cacheName)
	c.getURL = path //zstr.Concat("/", zrest.AppURLPrefix, path) // let's make image standalone and remove this dependency soon
	err := os.MkdirAll(c.workDir+cacheName, 0775|os.ModeDir)
	if err != nil {
		zlog.Error(err, zlog.FatalLevel, "zfilecaches.Init mkdir failed")
	}
	zrest.Handle(path, c)
	ztimer.RepeatNow(1800+200*rand.Float64(), func() bool {
		start := time.Now()
		dir := c.workDir + c.cacheName
		cutoff := time.Now().Add(-c.DeleteAfter)
		err := zfile.DeleteOldInSubFolders(dir, time.Millisecond*1, cutoff, c.DeleteRatio, func(p float32, count, total int) {
			fmt.Printf("DeleteCache: %s %d%% %d/%d\n", dir, int(p*100), count, total)
		})
		if err != nil {
			zlog.Error(err, "delete cache", c.cacheName)
		}
		zlog.Info("Deleted cache:", c.workDir+c.cacheName, time.Since(start))
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
func (c *Cache) CacheFromReader(reader io.Reader, ext, id string) (string, error) {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", zlog.Error(err, "read from reader")
	}
	return c.CacheFromData(data, ext, id)
}

// CacheFromData reads in data, using id as filename, or hash of data, and extension
// This string return is this id/hash, used to get a path or a url for fetching.
func (c *Cache) CacheFromData(data []byte, ext, id string) (string, error) {
	var err error
	h := fnv.New64a()
	h.Write(data)

	name := id
	if id == "" {
		hash := h.Sum64()
		name = fmt.Sprintf("%x", hash)
	}
	name += ext

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
	str := zstr.Concat("/", c.getURL, name) // zrest.AppURLPrefix,
	// zlog.Info("CACHE:", str)
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
