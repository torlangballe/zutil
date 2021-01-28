package zmarkdown

import (
	"os"
	"path/filepath"
	"regexp"

	blackfriday "github.com/torlangballe/blackfridayV2"
	"github.com/torlangballe/mdtopdf"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
	// import "github.com/mandolyte/mdtopdf"
)

// For embedded editor:
// https://reposhub.com/javascript/css/antonmedv-codejar.html
// https://godoc.org/github.com/russross/blackfriday

func ConvertToHTML(input, title string) (string, error) {
	params := blackfriday.HTMLRendererParameters{}
	params.Title = title
	params.Flags = blackfriday.CompletePage | blackfriday.HrefTargetBlank
	params.CSS = zrest.AppURLPrefix + "css/github-markdown.css"

	renderer := blackfriday.NewHTMLRenderer(params)
	return convertWithRenderer(input, title, renderer)
}

func convertWithRenderer(input, title string, renderer blackfriday.Renderer) (string, error) {
	output := blackfriday.Run([]byte(input),
		blackfriday.WithExtensions(blackfriday.CommonExtensions|blackfriday.HardLineBreak|blackfriday.AutoHeadingIDs),
		blackfriday.WithRenderer(renderer))
	return string(output), nil
}

func ConvertToPDF(input, title string, localFilePathPrefix string) (string, error) {
	tempFile := zfile.CreateTempFilePath(title + ".pdf")
	renderer := mdtopdf.NewPdfRenderer("", "", tempFile, "trace.log")
	renderer.LocalFilePathPrefix = localFilePathPrefix
	err := renderer.Process([]byte(input))
	if err != nil {
		return "", zlog.Error(err, "processing")
	}
	// zlog.Info("topdf:", zfile.Size(tempFile))
	spdf, err := zfile.ReadStringFromFile(tempFile)
	os.Remove(tempFile)
	return spdf, err
}

var linkReg = regexp.MustCompile(`\[[\w\s]+\]\(([\w/]+\.md)\)`)
var footReg = regexp.MustCompile(`\s*\[.+\]\:`)

func FlatttenMarkdown(pathPrefix string, chapters []string) (string, error) {
	var out, footers string
	for _, chapter := range chapters {
		atFooters := false
		err := zfile.ForAllFileLines(pathPrefix+chapter, func(s string) bool {
			if !atFooters && footReg.MatchString(s) {
				atFooters = true
			}
			if atFooters {
				footers += s + "\n"
				return true
			}
			snew := zstr.ReplaceAllCapturesFunc(linkReg, s, func(capture string, index int) string {
				fpath := capture
				// zstr.HasPrefix(fpath, "/", &fpath)
				if zhttp.StringStartsWithHTTPX(fpath) {
					return capture
				}
				_, file := filepath.Split(fpath)
				file = zfile.RemovedExtension(file)
				return "#" + file
			})
			out += snew + "\n"
			return true
		})
		if err != nil {
			return "", zlog.Error(err, "read lines from base")
		}
	}
	out += footers
	return out, nil
}
