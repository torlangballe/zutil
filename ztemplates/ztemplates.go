package ztemplates

import (
	"encoding/json"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
)

type Handler struct {
	templateLoaded zmap.LockMap[string, bool]
	basePath       string
	mainTemplate   *template.Template
	fileSystem     fs.FS
}

func NewHandler(base string, fileSystem fs.FS) (h *Handler) {
	h = new(Handler)
	h.basePath = base
	h.fileSystem = fileSystem
	return
}

func AddHandler(router *mux.Router, pattern string, handler *Handler, handlerFunc func(http.ResponseWriter, *http.Request, *Handler)) *mux.Route {
	return zrest.AddHandler(router, "templates/"+pattern, func(w http.ResponseWriter, req *http.Request) {
		zlog.Info("PlayTemplate:", req.Method, req.URL.Query().Get("Title"), req.RemoteAddr)
		handlerFunc(w, req, handler)
	})
}

func Marshal(v interface{}) template.JS {
	a, _ := json.MarshalIndent(v, "", "  ")
	return template.JS(a)
}

func TimeToYMDHSUtc(t time.Time) string {
	str := t.UTC().Format("2006-01-02 15:04")
	return str
}

func ToUrl(v interface{}) template.URL {
	s, _ := v.(string)
	return template.URL(s)
}

func ValueSelected(option string, value string) template.HTMLAttr {
	str := `value="` + option + `"`
	if option == value {
		str += " selected"
	}
	return template.HTMLAttr(str)
}

func Either(first bool, s1 string, s2 string) template.HTMLAttr {
	if first {
		return template.HTMLAttr(s1)
	}
	return template.HTMLAttr(s2)
}

func DisableIfZero(n int64) template.HTML {
	if n == 0 {
		return "disable"
	}
	return ""
}

func LockIcon(n int64) string {
	if n == 0 {
		return "lock.png"
	}
	return "unlock.png"
}

var fmap = map[string]interface{}{
	"marshal":  Marshal,
	"time2str": TimeToYMDHSUtc,
	"url":      ToUrl,
	"disable":  DisableIfZero,
	"lockicon": LockIcon,
	"valsel":   ValueSelected,
	"either":   Either,
}

func (h *Handler) loadTemplate(name string) error { // https://stackoverflow.com/questions/38686583/golang-parse-all-templates-in-directory-and-subdirectories
	if filepath.Ext(name) != ".gohtml" {
		return zlog.NewError("not template:", name)
	}
	_, loaded := h.templateLoaded.GetSet(name, true)
	if loaded {
		return nil
	}
	tpath := zstr.Concat("/", h.basePath, name)
	if h.mainTemplate == nil {
		h.mainTemplate = template.New("base")
	}
	data, errio := zfile.ReadBytesFromFileInFS(h.fileSystem, tpath)
	if errio != nil {
		return zlog.Error(errio, "ReadBytesFromFileInFS", tpath)
	}
	if len(data) == 0 {
		return zlog.Error(errio, "ReadBytesFromFileInFS data size 0", tpath)
	}
	zlog.Info("load template:", tpath, len(data))
	t := h.mainTemplate.New(name).Funcs(fmap)
	_, err := t.Parse(string(data))
	if err != nil {
		return zlog.Error(err, "(parse)")
	}
	return nil
}

func (h *Handler) ExecuteTemplate(w http.ResponseWriter, req *http.Request, dump bool, v interface{}) error {
	var out io.Writer
	out = w
	if dump {
		out = os.Stdout
	} else {
		zrest.AddCORSHeaders(w, req)
	}
	if req.Method == "OPTIONS" {
		return nil
	}
	path := req.URL.Path
	//	name := req.URL.Path[1:] + ".gohtml"
	zstr.HasPrefix(path, zrest.AppURLPrefix, &path)
	name := path + ".gohtml"
	// zlog.Info("ExecuteTemplate:", name)
	err := h.loadTemplate(name)
	if err != nil {
		return zrest.ReturnAndPrintError(w, req, http.StatusInternalServerError, "templates load error:", req.URL.Path, err)
	}
	err = h.mainTemplate.ExecuteTemplate(out, name, v)
	if err != nil {
		return zlog.Error(err, "exe error:", name, zlog.Full(v))
	}
	return nil
}
