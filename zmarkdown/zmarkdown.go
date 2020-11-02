package zmarkdown

import (
	"os"
	"path/filepath"
	"regexp"

	"github.com/torlangballe/blackfriday"
	"github.com/torlangballe/mdtopdf"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
	// import "github.com/mandolyte/mdtopdf"
)

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

// func FlatttenMarkdown(pathPrefix, basefilePath string, chapterPaths []string) (string, error) {
// 	var out string
// 	linkReg := regexp.MustCompile(`\[[\w\s]+\]\(([\w/]+\.md)\)`)
// 	footReg := regexp.MustCompile(`\s*\[.+\]\:`)

// 	atFooters := false
// 	err := zfile.ForAllFileLines(pathPrefix+basefilePath, func(s string) bool {
// 		if !atFooters && footReg.MatchString(s) {
// 			insertChapters(&out, pathPrefix, chapterPaths)
// 			atFooters = true
// 		}
// 		snew := zstr.ReplaceAllCapturesFunc(linkReg, s, func(capture string, index int) string {
// 			fpath := capture
// 			zstr.HasPrefix(fpath, "/", &fpath)
// 			zlog.Info("REG:", fpath, zstr.IndexOf(fpath, chapterPaths), chapterPaths)
// 			if zstr.IndexOf(fpath, chapterPaths) != -1 {
// 				_, file := filepath.Split(fpath)
// 				file = zfile.RemovedExtension(file)
// 				return "#" + file
// 			}
// 			return capture
// 		})
// 		out += snew + "\n"
// 		return true
// 	})
// 	if err != nil {
// 		return "", zlog.Error(err, "read lines from base")
// 	}
// 	return out, nil
// }

func insertChapters(out *string, pathPrefix, basefilePath string, skipChapters, chapterPaths []string) error {
	zlog.Info("insertChapters:", pathPrefix, basefilePath, skipChapters, chapterPaths)
	for _, c := range chapterPaths {
		str, err := FlatttenMarkdown(pathPrefix, c, zstr.UnionStringSet(skipChapters, chapterPaths))
		if err != nil {
			return zlog.Error(err, "flatten", c)
		}
		*out += str
	}
	return nil
}

func FlatttenMarkdown(pathPrefix, basefilePath string, skipChapters []string) (string, error) {
	var out string
	var chapterPaths []string

	linkReg := regexp.MustCompile(`\[[\w\s]+\]\(([\w/]+\.md)\)`)
	footReg := regexp.MustCompile(`\s*\[.+\]\:`)

	zlog.Info("flatten:", pathPrefix, basefilePath)
	atFooters := false
	err := zfile.ForAllFileLines(pathPrefix+basefilePath, func(s string) bool {
		if !atFooters && footReg.MatchString(s) {
			insertChapters(&out, pathPrefix, basefilePath, skipChapters, chapterPaths)
			atFooters = true
		}
		snew := zstr.ReplaceAllCapturesFunc(linkReg, s, func(capture string, index int) string {
			fpath := capture
			// zstr.HasPrefix(fpath, "/", &fpath)
			zlog.Info("REG:", fpath, zstr.IndexOf(fpath, chapterPaths), chapterPaths)
			if zhttp.StringStartsWithHTTPX(fpath) {
				return capture
			}
			if zstr.IndexOf(fpath, skipChapters) == -1 {
				zstr.AddToSet(&chapterPaths, fpath)
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
	return out, nil
}
