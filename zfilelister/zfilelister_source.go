//go:build server

package zfilelister

import (
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zfilecache"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
)

type FileServerCalls struct{}

type FileServer struct {
	IconCache *zfilecache.Cache
	// Token         string // If not empty, Header Authentication on requests needs to be with this token.
	router  *mux.Router
	folders map[string]string // storeName:baseFolder
}

const urlPrefix = "zfilelister-files"

var MainServer *FileServer

func NewFileServer(router *mux.Router, cacheBaseFolder string) *FileServer {
	s := &FileServer{}
	s.IconCache = zfilecache.Init(router, cacheBaseFolder, "caches/filelister-icons", "filelister-icons")
	s.IconCache.DeleteAfter = ztime.Day * 7
	s.IconCache.ServeEmptyImage = true
	s.IconCache.DeleteRatio = 0.1
	s.IconCache.InterceptServeFunc = s.interceptCache
	s.router = router
	s.folders = map[string]string{}
	return s
}

func (s *FileServer) AddFolder(baseFolder, storeName, servePath string) {
	folder := zfile.JoinPathParts(baseFolder, storeName)
	zfile.MakeDirAllIfNotExists(folder)
	urlBase := zfile.JoinPathParts(urlPrefix + storeName)
	s.folders[storeName] = baseFolder
	zlog.Info("zfilelister.AddFolder:", urlBase, servePath, folder)
	// zrest.AddFileHandler(s.router, urlBase, folder, s.handleServeFile)
}

func (FileServerCalls) GetDirectory(dirOpts DirOptions, paths *[]string) error {
	baseFolder := MainServer.folders[dirOpts.StoreName]
	folder := zfile.JoinPathParts(baseFolder, dirOpts.StoreName)
	walkOpts := zfile.WalkOptionGiveNameOnly | zfile.WalkOptionRecursive
	if dirOpts.ChooseFolders || dirOpts.FoldersOnly {
		walkOpts |= zfile.WalkOptionGiveFolders
	}
	var wildcards string
	if len(dirOpts.ExtensionsAllowed) != 0 {
		wildcards = "*" + strings.Join(dirOpts.ExtensionsAllowed, "\t*")
	}
	// zlog.Info("FileServerCalls.GetDir", folder, wildcards)
	err := zfile.Walk(folder, wildcards, walkOpts, func(fpath string, info os.FileInfo) error {
		// zlog.Info("FileServerCalls.GetDir2", fpath, dirOpts.ChooseFolders, dirOpts.FoldersOnly)
		if dirOpts.FoldersOnly && !info.IsDir() {
			return nil
		}
		if info.IsDir() {
			fpath += "/"
		}
		*paths = append(*paths, fpath)
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

/*
func (s *FileServer) handleServeFile(w http.ResponseWriter, filepath *string, urlpath string, req *http.Request) bool {
	// if req.Method == http.MethodOptions {
	// zlog.Info("zfilelister.handleServeFile options:", req.URL, req.Header.Get("Origin"))
	zrest.AddCORSHeaders(w, req)
	// return true
	// }
	zlog.Info("zfilelister.handleServeFile:", req.URL, *filepath)
	if s.Token != "" {
		token := req.URL.Query().Get("token")
		if s.Token != token {
			zrest.ReturnAndPrintError(w, req, http.StatusForbidden, "Token needed, not in header.", token, "!=", s.Token, *filepath)
			return false
		}
	}
	if zfile.IsFolder(*filepath) {
		zlog.Info("zfilelister.handleServeFile Dir:", *filepath)
		paths, err := zfile.GetFilesFromPath(*filepath, zfile.WalkOptionGiveNameOnly|zfile.WalkOptionGiveFolders)
		if err != nil {
			zrest.ReturnAndPrintError(w, req, http.StatusInternalServerError, err, *filepath)
			return true
		}
		if len(paths) != 0 {
			lines := strings.Join(paths, "\n")
			w.Write([]byte(lines))
		}
		return true
	}
	if !s.allowDownload {
		return true
	}
	return false // handle by zrest.AddFileHandler
}
*/

func (s *FileServer) interceptCache(w http.ResponseWriter, req *http.Request, file string) bool {
	zlog.Info("FS:intercept:", req.URL, file)
	return false
}
