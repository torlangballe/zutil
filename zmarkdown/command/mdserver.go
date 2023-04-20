package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmarkdown"
	"github.com/torlangballe/zutil/zrest"
)

type MDServer struct {
	router    *mux.Router // if ServeDirectories is true, it serves content list of directory
	dir       string
	pdfFiles  string
	title     string
	pdfURL    string
	htmlURL   string
	cssURL    string
	variables zdict.Dict
}

var server MDServer

// FilesRedirector's ServeHTTP serves everything in www, handling directories, * wildcards, and auto-translating .md (markdown) files to html
func (s MDServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	switch filepath.Ext(path) {
	case ".md":
		s.handleAsHTML(w, req)
		req.Body.Close()
	case ".pdf":
		s.handleAsPDF(w, req)
		req.Body.Close()
	default:
		file := filepath.Join(s.dir, req.URL.Path)
		zlog.Info("ServeFile:", file)
		http.ServeFile(w, req, file)
	}
}

func (s *MDServer) getVariables(req *http.Request) zdict.Dict {
	if len(s.variables) != 0 {
		return s.variables
	}
	vars := zdict.Dict{}
	for k, vs := range req.URL.Query() {
		vars[k] = vs[0]
	}
	return vars
}

func (s *MDServer) handleAsPDF(w http.ResponseWriter, req *http.Request) {
	fullmd, err := zmarkdown.FlattenMarkdown(s.dir, s.getFilesForPDF())
	if err != nil {
		zrest.ReturnAndPrintError(w, req, http.StatusInternalServerError, err, "building pdf guide")
		return
	}
	spdf, err := zmarkdown.ConvertToPDF(fullmd, s.title, s.dir, s.getVariables(req))
	if err != nil {
		zrest.ReturnAndPrintError(w, req, http.StatusInternalServerError, "error converting manual to pdf")
		return
	}
	w.Write([]byte(spdf))
}

func (s *MDServer) handleAsHTML(w http.ResponseWriter, req *http.Request) {
	file := filepath.Join(s.dir, req.URL.Path)
	zlog.Info("handleAsHTML", file)
	zmarkdown.ServeAsHTML(w, req, file, s.cssURL, s.getVariables(req))
}

func (s *MDServer) handleSetVariables(w http.ResponseWriter, req *http.Request) {
	vars := zdict.Dict{}
	err := json.NewDecoder(req.Body).Decode(&vars)
	if err != nil {
		zrest.ReturnAndPrintError(w, req, http.StatusBadRequest, err, "decoding variables")
		return
	}
	s.variables = vars
}

func (s *MDServer) getFilesForPDF() (files []string) {
	for _, pdf := range strings.Split(s.pdfFiles, ":") {
		file := filepath.Join(s.dir, pdf+".md")
		if zfile.NotExist(file) {
			zlog.Error(nil, "missing file:", file)
			return []string{}
		}
		files = append(files, file)
	}
	return
}

func main() {
	server.variables = zdict.Dict{}
	flag.StringVar(&server.dir, "dir", "", "path to directory of markdown.md files. Relative to server binary or absolute.")
	flag.StringVar(&server.pdfFiles, "pdfconcat", "", "colon-separated list of .md files to create pdf from, with no extension.")
	flag.StringVar(&server.pdfURL, "pdfurl", "/manual.pdf", "path to get full pdf.")
	flag.StringVar(&server.htmlURL, "htmlURL", "/doc/", "path to serve md files as html.")
	flag.StringVar(&server.cssURL, "cssurl", "/doc/github-markdown.css", "url to css file")
	flag.StringVar(&server.title, "title", "guide", "name of document.")
	port := flag.Int("port", 80, "port to serve from.")
	flag.Parse()

	server.dir = zfile.ExpandTildeInFilepath(server.dir)
	zmarkdown.InitCache(nil, server.dir+"doc/", "")
	http.Handle(server.htmlURL, server)
	zlog.Info("Serving markdown on:", server.htmlURL, "from:", server.dir, "port:", *port)
	err := http.ListenAndServe(fmt.Sprint(":", *port), nil)
	zlog.Error(err, "listen")
}
