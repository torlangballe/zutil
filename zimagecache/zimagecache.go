package zimagecache

import (
	"fmt"
	"hash/fnv"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
)

type Cache struct {
	workDir         string
	cacheName       string
	getUrl          string
	TokenValidHours int
}

func (c Cache) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	spath := req.URL.Path[1:]
	file := path.Join(c.workDir, spath)
	// zlog.Info("FileCache serve:", file)
	if !zfile.FileExist(file) {
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
	c.workDir = workDir
	c.cacheName = cacheName
	c.getUrl = urlPrefix + cacheName
	err := os.MkdirAll(c.workDir+cacheName, 0775|os.ModeDir)
	if err != nil {
		zlog.Log(err, zlog.FatalLevel, "zimages.Init mkdir failed")
	}
	http.Handle(c.getUrl, c)
	// fs := http.FileServer(http.Dir(workDir))
	// http.Handle(c.getUrl, fs)

	zlog.Info("init image cache:", c.workDir+c.cacheName, c.getUrl)
	return c
}

func (c *Cache) getToken() string {
	return ""
}

func (c *Cache) IsTokenValid(t string) bool {
	return true
}

// CacheImageFromReader reads bytes from reader and stores with stype png og jpeg, returning name.png/jpeg
// This name is used to get a path or a url for getting
func (c *Cache) CacheImageFromReader(reader io.Reader, stype string) (string, error) {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", zlog.Error(err, "read from reader")
	}
	return c.CacheImageFromData(data, stype)
}

func (c *Cache) CacheImageFromData(data []byte, stype string) (string, error) {
	var err error
	h := fnv.New64a()
	h.Write(data)

	hash := h.Sum64()
	name := fmt.Sprintf("%x.%s", hash, stype)

	path := c.GetCachePathForName(name)
	// outUrl := GetUrlForName(name)

	// zlog.Info("CacheImageFromReader path:", path)
	if zfile.FileExist(path) {
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
	str := c.getUrl + name
	if c.TokenValidHours != 0 {
		str += "?token=" + c.getToken()
	}
	return str
}

func (c *Cache) SetCacheCommentForName(name, comment string) error {
	if zrest.RunningOnServer {
		return nil
	}
	err := zfile.SetComment(c.GetCachePathForName(name), comment)
	if err != nil {
		return zlog.Error(err, "set comment")
	}
	return nil
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
