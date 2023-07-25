package zmarkdown

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	blackfriday "github.com/torlangballe/blackfridayV2"
	"github.com/torlangballe/mdtopdf"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
)

// For embedded editor:
// https://reposhub.com/javascript/css/antonmedv-codejar.html
// https://godoc.org/github.com/russross/blackfriday

func ConvertToHTML(filepath, title, cssURL string, variables zdict.Dict) (string, error) {
	t := newTemplater()
	input := t.Preprocess(filepath, title, variables)
	params := blackfriday.HTMLRendererParameters{}
	params.Title = title
	params.Flags = blackfriday.CompletePage | blackfriday.HrefTargetBlank
	if cssURL == "" {
		params.CSS = zrest.AppURLPrefix + "css/github-markdown.css"
	} else {
		params.CSS = cssURL
	}
	renderer := blackfriday.NewHTMLRenderer(params)
	output := blackfriday.Run([]byte(input),
		blackfriday.WithExtensions(extensions|blackfriday.AutoHeadingIDs),
		blackfriday.WithRenderer(renderer))
	return string(output), nil
}

var extensions = blackfriday.NoIntraEmphasis | blackfriday.Tables | blackfriday.FencedCode |
	blackfriday.Autolink | blackfriday.Strikethrough | blackfriday.SpaceHeadings | blackfriday.HeadingIDs |
	blackfriday.BackslashLineBreak | blackfriday.DefinitionLists | blackfriday.HardLineBreak

func ConvertToPDF(filepath, title, localFilePathPrefix string, variables zdict.Dict) (string, error) {
	t := newTemplater()
	t.DPI = 300
	input := t.Preprocess(filepath, title, variables)
	tempFile := zfile.CreateTempFilePath(title + ".pdf")
	renderer := mdtopdf.NewPdfRenderer("", "", tempFile, "trace.log")
	renderer.LocalFilePathPrefix = localFilePathPrefix
	renderer.LocalImagePathAlternativePrefix = Cache.GetWorkDirectoryStart()
	zlog.Info("ConvertToPDF:", renderer.LocalImagePathAlternativePrefix)
	err := renderer.Process([]byte(input), blackfriday.WithExtensions(extensions)) //blackfriday.HeadingIDs))
	if err != nil {
		return "", zlog.Error(err, "processing", zlog.CallingStackString())
	}
	spdf, err := zfile.ReadStringFromFile(tempFile)
	os.Remove(tempFile)
	return spdf, err
}

// var linkReg = regexp.MustCompile(`\[[\w\s]+\]\(([\w/]+\.md)\)`)
var linkFileReg = regexp.MustCompile(`\[[\s\w\*\.\:]+\]\(([\w/]+\.md)\)`)
var linkInterReg = regexp.MustCompile(`\[[\w\s]+\]\((#[\w/]+)\)`)
var footReg = regexp.MustCompile(`\s*\[.+\]\:`)
var headerReg = regexp.MustCompile(`^#{1,6}\s*(.+)`)
var headerReplacer = strings.NewReplacer(
	" ", "_",
	"#", "",
	".", "_",
)

func headerToAnchorID(header string) string {
	header = strings.ToLower(header)
	return zstr.ReplaceWithFunc(header, func(r rune) string {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r == '_' || r == '-' {
			return string(r)
		}
		return ""
	})
}

func anchorFromFileAndAnchor(file, anchor string) string {
	file = headerReplacer.Replace(file)
	return zstr.Concat("_", file, anchor)
}

type content struct {
	level   int
	title   string
	anchor  string
	chapter string
}

func getNiceHeader(header *string, rest *string) bool {
	*header = strings.TrimSpace(*header)
	// if strings.HasPrefix(*header, "!") {
	// 	return false
	// }
	*header = zstr.HeadUntil(*header, "![")
	if rest == nil || *rest == "" {
		*header = zstr.HeadUntil(*header, "[")
	}
	return true
}

// type chapterAnchor struct {
// 	chapter string
// 	anchor  string
// }

func getTableOfContents(contents []content) string {
	table := "\\\n### Table of Contents\n\n\\\n\\\n"
	for _, c := range contents {
		table += "**[" + c.title + "](#" + c.anchor + ")**\n\n\\\n"
	}
	return table + "\\\n\\\n"
}

func FlattenMarkdown(pathPrefix string, chapters []string, tableOfContents bool) (string, error) {
	var out, footers string
	var contents []content

	// var topFileAnchors []chapterAnchor

	// str := `### ![open](open.png) Open prefs`
	// zstr.ReplaceAllCapturesFunc(headerReg, str, func(capture string, index int) string {
	// 	zlog.Info("Replace:", capture)
	// 	return "xxx"
	// })
	// return "", nil
	for _, chapter := range chapters {
		zfile.ForAllFileLines(pathPrefix+chapter, false, func(s string) bool {
			header := headerReg.FindString(s)
			if header != "" {
				i := strings.IndexFunc(header, func(r rune) bool {
					return r != '#'
				})
				if !getNiceHeader(&header, nil) {
					return true
				}
				var c content
				c.level = i
				header = strings.TrimSpace(header[i:])
				id := headerToAnchorID(header)
				c.title = header
				c.anchor = anchorFromFileAndAnchor(chapter, id)
				c.chapter = chapter
				contents = append(contents, c)
				// topFileAnchors = append(topFileAnchors, c.anchor})
				// zlog.Info("ANCH:", topFileAnchors[chapter], chapter, id)
				return false
			}
			return true
		})
	}
	if tableOfContents {
		out = getTableOfContents(contents)
	}
	for _, chapter := range chapters {
		// atFooters := false
		err := zfile.ForAllFileLines(pathPrefix+chapter, false, func(s string) bool {
			// if !atFooters && footReg.MatchString(s) {
			// 	atFooters = true
			// }
			// if atFooters {
			// 	footers += s + "\n"
			// 	return true
			// }
			snew := zstr.ReplaceAllCapturesFunc(linkInterReg, s, func(capture string, index int) string {
				// zlog.Info("replace inter:", s, capture, index)
				file, anchor := zstr.SplitInTwo(capture, "#")
				if file == "" {
					file = chapter
					anchor = capture
					zstr.HasPrefix(anchor, "#", &anchor)
				}
				link := "#" + anchorFromFileAndAnchor(file, anchor)
				return link
			})
			snew = zstr.ReplaceAllCapturesFunc(linkFileReg, snew, func(capture string, index int) string {
				// if zhttp.HasURLScheme(capture) {
				// 	return capture
				// }
				// if !strings.Contains(capture, "#") {
				// zlog.Info("replace md file:", chapter, capture, snew)
				for _, c := range contents {
					if c.chapter == capture {
						link := "#" + c.anchor
						return link
					}
				}
				return ""
				// }
				// file, anchor := zstr.SplitInTwo(capture, "#")
				// link := "#" + anchorFromFileAndAnchor(file, anchor)
				// return link
			})
			snew = zstr.ReplaceAllCapturesFunc(headerReg, snew, func(capture string, index int) string {
				// zlog.Info("replace headers:", snew, capture, index)
				if strings.HasPrefix(capture, "!") {
					return capture
				}
				// var titleReg = regexp.MustCompile(`\[([(\w\s]+)\]\(\S+\)(.+)`)
				// parts := zstr.GetAllCaptures(titleReg, capture)
				// if len(parts) > 1 {
				// 	zlog.Info("PARTS:", zstr.Spaced(zstr.StringsToAnySlice(parts)...))
				// 	return zstr.Spaced(zstr.StringsToAnySlice(parts)...)
				// }
				var rest string
				getNiceHeader(&capture, &rest)
				id := headerToAnchorID(capture)
				anchor := anchorFromFileAndAnchor(chapter, id)
				nstr := fmt.Sprintf("%s {#%s} %s", capture, anchor, rest)
				return nstr
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

func ServeAsHTML(w http.ResponseWriter, req *http.Request, filepath, cssURL string, variables zdict.Dict) {
	defer req.Body.Close()
	html, err := ConvertToHTML(filepath, req.URL.Path, cssURL, variables)
	if err != nil {
		zrest.ReturnAndPrintError(w, req, http.StatusInternalServerError, err, "convert")
		return
	}
	zrest.AddCORSHeaders(w, req)
	io.WriteString(w, html)
}
