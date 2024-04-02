//go:build server

package zfilelister

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zfilecache"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
)

type FilServerCaller struct{}

type FileServer struct {
	IconSize  zgeo.Size
	IconCache *zfilecache.Cache
	// Token         string // If not empty, Header Authentication on requests needs to be with this token.
	router  *mux.Router
	folders map[string]Options // baseFolder:Option
}

var MainServer *FileServer

func NewFileServer(router *mux.Router, allowDownload bool, name, baseFolder, servePath string, size zgeo.Size) *FileServer {
	s := &FileServer{}
	s.IconCache = zfilecache.Init(router, baseFolder, "caches/filelister-icons/"+name, "filelister-icons/"+name)
	s.IconCache.DeleteAfter = ztime.Day * 7
	s.IconCache.ServeEmptyImage = true
	s.IconCache.DeleteRatio = 0.1
	s.IconCache.InterceptServeFunc = s.interceptCache
	s.router = router
	s.folders = map[string]Options{}
	return s
}

func (s *FileServer) AddFolder(baseFolder, servePath string, opt Options) {
	folder := zfile.JoinPathParts(baseFolder, opt.StoreName)
	zfile.MakeDirAllIfNotExists(folder)
	urlBase := "zfilelister-files/" + opt.StoreName
	zlog.Info("zfilelister.AddFolder:", urlBase, servePath, folder)
	// zrest.AddFileHandler(s.router, urlBase, folder, s.handleServeFile)
}

func (FilServerCaller) GetDirectory(dirOpts Options, paths *[]string) {

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
