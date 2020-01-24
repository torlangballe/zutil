package zimages

import (
	"fmt"
	"hash/fnv"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/torlangballe/zutil/zcommand"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zredis"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/ztime"
)

var storagePath string
var getUrl string
var redisPool *redis.Pool

func InitCache(workDir, urlPrefix string, rpool *redis.Pool) {
	redisPool = rpool
	storagePath = workDir + "images/cache/"
	getUrl = urlPrefix + "images/cache/"
	err := os.MkdirAll(storagePath, 0775|os.ModeDir)
	if err != nil {
		zlog.Log(err, zlog.FatalLevel, "images.Init mkdir failed")
	}
}

func getToken() string {
	var token string
	key := "images.url.token"
	got, err := zredis.Get(redisPool, &token, key)
	if err != nil {
		zlog.Error(err, "redis get")
		return ""
	}
	if !got {
		token = fmt.Sprintf("%x", rand.Int31())
		err = zredis.Put(redisPool, key, ztime.Day, token)
		if err != nil {
			zlog.Error(err, "redis put")
			return ""
		}
	}
	return token
}

func IsTokenValid(t string) bool {
	return t == getToken()
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
	if redisPool != nil {
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
