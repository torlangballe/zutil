package ztemplates

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/torlangballe/zutil/urest"
	"github.com/torlangballe/zutil/zlog"

	"github.com/gorilla/mux"
)

type Handler struct {
	templates     map[string]*template.Template
	baseDirectory string
	mainTemplate  *template.Template
}

func NewHandler(base string) (h *Handler) {
	h = new(Handler)
	h.templates = map[string]*template.Template{}
	h.baseDirectory = base

	return
}

type HandlerFunc func(http.ResponseWriter, *http.Request, *Handler)

func AddHandler(r *mux.Router, pattern string, handler *Handler, handlerFunc HandlerFunc) *mux.Route {
	return r.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
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

func (h *Handler) GetTemplate(path string) (t *template.Template, err error) {
	name := path[1:] + ".gohtml"
	path = h.baseDirectory + "www/templates/" + name
	/*
		t = templates[path]
		if t != nil {
			return
		}
	*/
	//	fmt.Println("get template:", path, ufile.DoesFileExist(path))
	t, err = template.New(name).Funcs(fmap).ParseFiles(path)
	if err != nil {
		fmt.Println("getTemplate err:", err)
		return
	}
	return
}

func (h *Handler) LoadTemplates() (err error) { // https://stackoverflow.com/questions/38686583/golang-parse-all-templates-in-directory-and-subdirectories
	spath := h.baseDirectory + "www/templates/"
	root := template.New("base")
	index := len(spath)
	filepath.Walk(spath, func(fpath string, info os.FileInfo, err error) error {
		if err == nil && spath != fpath && filepath.Ext(fpath) == ".gohtml" {
			data, errio := ioutil.ReadFile(fpath)
			if errio != nil {
				return zlog.Error(errio, "readfile")
			}
			name := fpath[index:]
			t := root.New(name).Funcs(fmap)
			t, err = t.Parse(string(data))
			if err != nil {
				return zlog.Error(err, "parse")
			}
		}
		return nil
	})
	h.mainTemplate = root
	if err != nil {
		return
	}
	return
}

func (h *Handler) ExecuteTemplate(w http.ResponseWriter, req *http.Request, dump bool, v interface{}) bool {
	var out io.Writer
	out = w
	if dump {
		out = os.Stdout
	} else {
		zrest.AddCORSHeaders(w, req)
	}
	err := h.LoadTemplates()
	if err != nil {
		zrest.ReturnAndPrintError(w, req, http.StatusInternalServerError, "templates get error:", req.URL.Path, err)
		return false
	}
	name := req.URL.Path[1:] + ".gohtml"
	err = h.mainTemplate.ExecuteTemplate(out, name, v)
	if err != nil {
		fmt.Println("Web Exe error:", err)
		return false
	}
	return true
}
