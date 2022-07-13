package zmarkdown

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/go-latex/latex/drawtex/drawimg"
	"github.com/go-latex/latex/mtex"
	"github.com/goccy/go-graphviz"
	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zfilecache"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

type Templater struct {
	DPI         int
	TeXFontSize float64
}

const (
	TexType = "text"
	DotType = "dot"
)

var Cache *zfilecache.Cache

func InitCache(router *mux.Router, workDir, urlPrefix string) {
	Cache = zfilecache.Init(router, workDir, urlPrefix, "zmd-rendercache")
	Cache.NestInHashFolders = false
}

func getHash(str string, stype string, scale float64) string {
	return zstr.HashTo64Hex(fmt.Sprintf("%s-%s-%g", str, stype, scale)) + ".png"
}

func errToStr(err error, title, desc string) string {
	return fmt.Sprint(title, "@", desc, ": ", err)
}

func (t *Templater) processTex(fontScale float64, title, teXStr string) (output string) {
	zlog.Info("processTex:", title, fontScale, "\n", teXStr)
	cacheID := getHash(teXStr, TexType, fontScale)
	if !Cache.IsCached(cacheID) {
		var buf bytes.Buffer
		// reader, writer := io.Pipe()
		dest := drawimg.NewRenderer(&buf)
		err := mtex.Render(dest, teXStr, t.TeXFontSize*fontScale, float64(t.DPI), nil)
		if err != nil {
			return errToStr(err, title, "render")
		}
		_, err = Cache.CacheFromData(buf.Bytes(), cacheID)
		if err != nil {
			return errToStr(err, title, "get-from-cache")
		}
	}
	return getImageMarkdownFromCacheID(title, cacheID)
}

func (t *Templater) processDot(scale float64, title, dotStr string) string {
	// https://renenyffenegger.ch/notes/tools/Graphviz/examples/index
	zlog.Info("processDot:", title, scale, "\n", dotStr)
	cacheID := getHash(dotStr, DotType, scale)
	if !Cache.IsCached(cacheID) {
		graph, err := graphviz.ParseBytes([]byte(dotStr))
		if err != nil {
			return errToStr(err, title, "parse-bytes")
		}
		graph = graph.SetDPI(96 * scale) // can't get SetScale to work, or any attributes in dot graph, so hacking dpi, which is default 96
		var buf bytes.Buffer
		g := graphviz.New()
		err = g.Render(graph, graphviz.PNG, &buf)
		if err != nil {
			return errToStr(err, title, "render")
		}
		_, err = Cache.CacheFromData(buf.Bytes(), cacheID)
		if err != nil {
			return errToStr(err, title, "get-from-cache")
		}
	}
	return getImageMarkdownFromCacheID(title, cacheID)
}

func getImageMarkdownFromCacheID(title, cacheID string) string {
	surl := Cache.GetURLForName(cacheID)
	return fmt.Sprintf("![%s](%s)", title, surl)
}

func (t *Templater) Preprocess(title, markdownText string, variables zdict.Dict) string {
	zlog.Assert(Cache != nil, "cache")
	var buf bytes.Buffer
	funcMap := template.FuncMap{
		"tex": t.processTex,
		"dot": t.processDot,
	}

	template, err := template.New(title).Funcs(funcMap).Parse(markdownText)
	if err != nil {
		return errToStr(err, title, "template-parse")
	}
	err = template.Execute(&buf, variables)
	if err != nil {
		return errToStr(err, title, "template-execute")
	}
	return buf.String()
}