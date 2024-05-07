//go:build server

package zfilelister

import (
	"embed"
	"io"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zui/zapp"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zfilecache"
	"github.com/torlangballe/zutil/zfiles"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zslice"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
)

//go:embed images
var iconsFS embed.FS

type FileServerCalls struct{}

type FileServer struct {
	IconCache *zfilecache.Cache
	router    *mux.Router
	folders   map[string]string // storeName:baseFolder
}

const urlPrefix = "zfilelister-files"

var (
	MainServer   *FileServer
	addedIconsFS bool
)

func NewFileServer(router *mux.Router, cacheBaseFolder string) *FileServer {
	if !addedIconsFS {
		zapp.AllWebFS.Add(iconsFS, "zfilelister-icons."+cacheBaseFolder)
	}
	s := &FileServer{}
	s.IconCache = zfilecache.Init(router, cacheBaseFolder, "caches", "filelister-icons")
	s.IconCache.DeleteAfter = ztime.Day * 7
	s.IconCache.ServeEmptyImage = true
	s.IconCache.DeleteRatio = 0.1
	s.IconCache.InterceptServeFunc = s.interceptCache
	s.IconCache.NestInHashFolders = false
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
	folder := zfile.JoinPathParts(baseFolder, dirOpts.StoreName, dirOpts.PathStub)
	walkOpts := zfile.WalkOptionGiveNameOnly
	if dirOpts.ChooseFolders || dirOpts.FoldersOnly {
		walkOpts |= zfile.WalkOptionGiveFolders
	}
	var wildcards string
	if len(dirOpts.ExtensionsAllowed) != 0 {
		wildcards = "*" + strings.Join(dirOpts.ExtensionsAllowed, "\t*")
	}
	zlog.Info("FileServerCalls.GetDir", folder, wildcards)
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
	sort.Strings(*paths)
	if err != nil {
		return err
	}
	return nil
}

func (FileServerCalls) ExpandFilePathsFromPicked(dirOpts DirOptions, paths *[]string) error {
	var all []string
	for _, f := range dirOpts.PickedPaths {
		if strings.HasSuffix(f, "/") {
			files, err := zfiles.GetFilesFromPath(f, "", zfile.WalkOptionRecursive)
			zlog.OnError(err, f)
			all = append(all, files...)
		} else {
			all = append(all, f)
		}
	}
	if dirOpts.MaxFiles == 0 {
		*paths = all
		return nil
	}
	for dirOpts.MaxFiles > 0 && len(all) > 0 {
		f, i := zslice.Random(all)
		zslice.RemoveAt(&all, i)
		*paths = append(*paths, f)
		dirOpts.MaxFiles--
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

func (s *FileServer) interceptCache(w http.ResponseWriter, req *http.Request, fpath *string) bool {
	const prefix = "images/zcore/zfilelister/icons/"
	// zlog.Info("FS:intercept:", req.URL, *fpath)
	ext := path.Ext(*fpath)
	var path string
	if ext == "" && strings.HasSuffix(req.URL.Path, "/") {
		ext = ".folder_"
	} else {
		if zstr.StringsContain(zfile.ImageExtensions, ext) {
			return s.serveThumb(w, req, fpath)
		}
	}
	docPath := prefix + "document.png"
	if ext == "" {
		path = docPath
	} else {
		path = prefix + ext[1:] + ".png"
	}
	file, err := iconsFS.Open(path)
	zlog.Info("FS:intercept:", path, req.URL, *fpath, err)
	if err != nil && path != docPath {
		file, err = iconsFS.Open(docPath)
	}
	if err == nil {
		_, err = io.Copy(w, file)
		zlog.OnError(err)
		return true
	}
	return true
}

func (s *FileServer) serveThumb(w http.ResponseWriter, req *http.Request, fpath *string) bool {
	var rest string
	prefix := zfile.JoinPathParts(s.IconCache.WorkDir, cachePrefix) + "/"
	// zlog.Info("serveThumb1:", fpath)

	size, _ := zgeo.SizeFromString(req.URL.Query().Get("size"))
	if zstr.HasPrefix(*fpath, prefix, &rest) {
		name, _ := zstr.SplitInTwo(rest, "/")
		baseFolder := s.folders[name]
		imagePath := zfile.JoinPathParts(baseFolder, rest)
		// img, format, err := zimage.GoImageFromFile(file)
		zlog.Info("serveThumb1:", rest, size)

		zlog.Info("serveThumb:", imagePath, "toicon:", *fpath)
	}
	return false
}
