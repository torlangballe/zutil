//go:build server

package zrest

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

type FilesRedirector struct {
	Overrides        []func(w http.ResponseWriter, req *http.Request, filepath string) bool // Override is a method to handle special cases of files, return true if handled
	ServeDirectories bool
	Router           *mux.Router // if ServeDirectories is true, it serves content list of directory

	embeddedFileSystems []embed.FS
}

func (f *FilesRedirector) AddEmbededFS(fs embed.FS) {
	f.embeddedFileSystems = append(f.embeddedFileSystems, fs)
}

func (f *FilesRedirector) AllembeddedFileSystems() []fs.FS {
	var efs []fs.FS
	for _, e := range f.embeddedFileSystems {
		efs = append(efs, e)
	}
	return efs
}

// FilesRedirector's ServeHTTP serves everything in www, handling directories, * wildcards, and auto-translating .md (markdown) files to html
func (f *FilesRedirector) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	const filePathPrefix = StaticFolder + "/"
	spath := req.URL.Path
	var redirectToDir bool
	if spath == strings.TrimRight(AppURLPrefix, "/") {
		redirectToDir = true
		spath += "/"
	}
	// fmt.Println(AppURLPrefix, "FilesRedir1:", req.URL.Path, spath, strings.Trim(AppURLPrefix, "/"))
	zstr.HasPrefix(spath, AppURLPrefix, &spath)
	fmt.Println(AppURLPrefix, "FilesRedir2:", req.URL.Path, spath, strings.Trim(AppURLPrefix, "/"))
	filepath := path.Join(filePathPrefix, spath)
	for _, o := range f.Overrides {
		if o(w, req, filepath) {
			req.Body.Close()
			return
		}
	}
	if filepath == StaticFolder {
		filepath = StaticFolder + "/index.html"
	}
	fmt.Println(AppURLPrefix, "FilesRedir3:", req.URL.Path, spath, filepath)

	if filepath == StaticFolder+"/main.wasm.gz" {
		zlog.Info("Serve WASM.gz:", filepath)
		// If we are serving the gzip'ed wasm file, set encoding to gzip and type to wasm
		w.Header().Set("Content-Type", "application/wasm")
		w.Header().Set("Content-Encoding", "gzip")
	}
	if zfile.Exists(filepath) {
		// zlog.Info("FilesServe:", req.URL.Path, filepath, zfile.Exists(filepath))
		http.ServeFile(w, req, filepath)
		return
	}
	if redirectToDir {
		newPath := AppURLPrefix
		// zlog.Info("Serve embed:", spath)
		if q := req.URL.RawQuery; q != "" {
			newPath += "?" + q
		}
		w.Header().Set("Location", newPath)
		w.WriteHeader(http.StatusMovedPermanently)
		req.Body.Close()
		return
	}
	// zlog.Info("FilesRedir2:", req.URL.Path, spath)

	if spath == "" { // hack to replicate how http.ServeFile serves index.html if serving empty folder at root level
		spath = "index.html"
	}
	var data []byte
	var err error
	for _, w := range f.embeddedFileSystems {
		data, err = w.ReadFile(StaticFolder + "/" + spath)
		// zlog.Info("EmbedRead:", StaticFolder+"/"+spath, data != nil, err)
		if data != nil {
			break
		}
	}
	// zlog.Info("FSREAD:", StaticFolder+"/"+spath, err, len(data), req.URL.String())
	req.Body.Close()
	if err == nil {
		_, err := w.Write(data)
		if err != nil {
			zlog.Error(err, "write to ResponseWriter from embedded")
		}
		return
	}
	// zlog.Info("Serve app:", path, filepath)
}
