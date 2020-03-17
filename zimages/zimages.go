package zimages

import (
	"fmt"
	"hash/fnv"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zcommand"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
)

var storagePath string
var getUrl string
var TokenValidHours int

func InitCache(workDir, urlPrefix, cacheName string) {
	if !strings.HasPrefix(urlPrefix, "/") {
		zlog.Error(nil, "url should start with /", urlPrefix)
	}
	if !strings.HasSuffix(urlPrefix, "/") {
		zlog.Fatal(nil, "url should end with /", urlPrefix)
	}
	storagePath = workDir + cacheName
	getUrl = urlPrefix + cacheName
	err := os.MkdirAll(storagePath, 0775|os.ModeDir)
	if err != nil {
		zlog.Log(err, zlog.FatalLevel, "zimages.Init mkdir failed")
	}
	fs := http.FileServer(http.Dir(workDir))
	http.Handle(getUrl, fs)

	fmt.Println("init image cache:", storagePath, getUrl)
}

func getToken() string {
	return ""
}

func IsTokenValid(t string) bool {
	return true
}

// StoreImageFromReader reads bytes from reader and stores with stype png og jpeg, returning name.png/jpeg
// This name is used to get a path or a url for getting
func CacheImageFromReader(reader io.Reader, stype string) (string, error) {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", zlog.Error(err, "read from reader")
	}
	h := fnv.New64a()
	h.Write(data)

	hash := h.Sum64()
	name := fmt.Sprintf("%x.%s", hash, stype)

	path := GetCachePathForName(name)
	// outUrl := GetUrlForName(name)

	if zfile.DoesFileExist(path) {
		err = zfile.SetModified(path, time.Now())
		return name, err
	}
	file, err := os.Create(path)
	if err != nil {
		return "", zlog.Error(err, "create file", path)
	}
	_, err = file.Write(data)
	if err != nil {
		return "", zlog.Error(err, "write to file")
	}
	return name, nil
}

func GetCachePathForName(name string) string {
	return filepath.Join(storagePath, name)
}

func GetCacheUrlForName(name string) string {
	str := getUrl + name
	if TokenValidHours != 0 {
		str += "?token=" + getToken()
	}
	return str
}

func SetCacheCommentForName(name, comment string) error {
	if zrest.RunningOnServer {
		return nil
	}
	_, err := zcommand.SetMacComment(GetCachePathForName(name), comment)
	if err != nil {
		return zlog.Error(err, "set comment")
	}
	return nil
}

func CheckCacheToken(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t := req.URL.Query().Get("token")
		if !IsTokenValid(t) {
			zrest.ReturnError(w, req, "bad token for get image", http.StatusBadRequest)
			return
		}
		h.ServeHTTP(w, req)
	})
}
