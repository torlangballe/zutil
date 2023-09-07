package zmarkdown

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	// "github.com/go-latex/latex/drawtex/drawimg"
	// "github.com/go-latex/latex/mtex"
	// "github.com/goccy/go-graphviz"

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
)

type Templater struct {
	DPI         int
	TeXFontSize float64
}

const (
	TexType = "text"
	DotType = "dot"
)

func newTemplater() *Templater {
	t := &Templater{}
	t.TeXFontSize = 14
	t.DPI = 144
	return t
}

func errToStr(err error, title, desc string) string {
	return fmt.Sprint(title, "@", desc, ": ", err)
}

// func (t *Templater) processTex(fontScale float64, title, teXStr string) (output string) {
// 	zlog.Info("processTex:", title, fontScale, "\n", teXStr)
// 	cacheID := getHash(teXStr, TexType, fontScale)
// 	if !Cache.IsCached(cacheID) {
// 		var buf bytes.Buffer
// 		// reader, writer := io.Pipe()
// 		dest := drawimg.NewRenderer(&buf)
// 		err := mtex.Render(dest, teXStr, t.TeXFontSize*fontScale, float64(t.DPI), nil)
// 		if err != nil {
// 			return errToStr(err, title, "render")
// 		}
// 		_, err = Cache.CacheFromData(buf.Bytes(), cacheID)
// 		if err != nil {
// 			return errToStr(err, title, "get-from-cache")
// 		}
// 	}
// 	return getImageMarkdownFromCacheID(title, cacheID)
// }

// func (t *Templater) processDot(scale float64, title, dotStr string) string {
// 	// https://renenyffenegger.ch/notes/tools/Graphviz/examples/index
// 	zlog.Info("processDot:", title, scale, "\n", dotStr)
// 	cacheID := getHash(dotStr, DotType, scale)
// 	if !Cache.IsCached(cacheID) {
// 		graph, err := graphviz.ParseBytes([]byte(dotStr))
// 		if err != nil {
// 			return errToStr(err, title, "parse-bytes")
// 		}
// 		graph = graph.SetDPI(96 * scale) // can't get SetScale to work, or any attributes in dot graph, so hacking dpi, which is default 96
// 		var buf bytes.Buffer
// 		g := graphviz.New()
// 		err = g.Render(graph, graphviz.PNG, &buf)
// 		if err != nil {
// 			return errToStr(err, title, "render")
// 		}
// 		_, err = Cache.CacheFromData(buf.Bytes(), cacheID)
// 		if err != nil {
// 			return errToStr(err, title, "get-from-cache")
// 		}
// 	}
// 	return getImageMarkdownFromCacheID(title, cacheID)
// }

// func getImageMarkdownFromCacheID(title, cacheID string) string {
// 	surl := Cache.GetURLForName(cacheID)
// 	return fmt.Sprintf("![%s](%s)", title, surl)
// }

func (t *Templater) Preprocess(m *MarkdownConverter, markdownText, title string) string {
	// zlog.Assert(Cache != nil, "cache")
	var buf bytes.Buffer
	funcMap := template.FuncMap{
		// "tex": t.processTex,
		// "dot": t.processDot,
	}

	template, err := template.New(title).Funcs(funcMap).Parse(markdownText)
	if err != nil {
		return errToStr(err, title, "template-parse")
	}
	for _, name := range m.PartNames {
		if !strings.HasSuffix(name, "shared.md") {
			continue
		}
		input, _ := zfile.ReadStringFromFileInFS(m.FileSystem, name)
		_, err := template.New(name).Funcs(funcMap).Parse(input)
		if zlog.OnError(err, name, "parse sub-template") {
			continue
		}
	}
	err = template.Execute(&buf, m.Variables)
	if err != nil {
		zlog.Error(err, "markdown execute", m.Dir)
		return errToStr(err, title, "template-execute")
	}
	return buf.String()
}
