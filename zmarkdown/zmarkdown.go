package zmarkdown

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	blackfriday "github.com/torlangballe/blackfridayV2"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
)

// For embedded editor:
// https://reposhub.com/javascript/css/antonmedv-codejar.html
// https://godoc.org/github.com/russross/blackfriday

type OutputType string

const (
	OutputPDF  OutputType = "pdf"
	OutputMD   OutputType = "md"
	OutputHTML OutputType = "html"

	SharedPageSuffix = ".shared.md"
)

type MarkdownConverter struct {
	PartNames       []string
	Dir             string
	Variables       zdict.Dict
	FileSystem      fs.FS
	TableOfContents bool
}

func (m *MarkdownConverter) ConvertFromHTMLToPDF(w io.Writer, surl, title string) error {
	taskCtx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	var pdfBuffer []byte
	err := chromedp.Run(taskCtx, pdfGrabber(w, surl, "body", &pdfBuffer))
	if err != nil {
		return zlog.Error(err)
	}
	// err = ioutil.WriteFile("coolsite.pdf", pdfBuffer, 0644)
	// if err != nil {
	// 	return zlog.Error(err)
	// }

	// // os.Remove(tempFile)
	// return string(pdfBuffer), nil
	return nil
}

func pdfGrabber(w io.Writer, url string, sel string, res *[]byte) chromedp.Tasks {
	start := time.Now()
	return chromedp.Tasks{
		emulation.SetUserAgentOverride("WebScraper 1.0"),
		chromedp.Navigate(url),
		// wait for footer element is visible (ie, page is loaded)
		// chromedp.ScrollIntoView(`footer`),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		// chromedp.Text(`h1`, &res, chromedp.NodeVisible, chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			buf, _, err := page.PrintToPDF().WithPrintBackground(true).Do(ctx)
			if err != nil {
				return err
			}
			//			w.Write(buf)
			*res = buf
			//fmt.Printf("h1 contains: '%s'\n", res)
			fmt.Printf("\nTook: %f secs\n", time.Since(start).Seconds())
			return nil
		}),
	}
}

// google-chrome-stable

func (m *MarkdownConverter) ConvertToHTML(w io.Writer, name string) error {
	fullmd, err := m.Flatten()
	if err != nil {
		return zlog.Error(err, "building doc", name)
	}
	return m.ConvertToHTMLFromString(w, fullmd, name)
}

func (m *MarkdownConverter) ConvertToHTMLFromString(w io.Writer, fullmd, name string) error {
	var extensions = blackfriday.NoIntraEmphasis | blackfriday.Tables | blackfriday.FencedCode |
		blackfriday.Autolink | blackfriday.Strikethrough | blackfriday.SpaceHeadings | blackfriday.HeadingIDs |
		blackfriday.BackslashLineBreak | blackfriday.DefinitionLists | blackfriday.HardLineBreak

	t := newTemplater()
	input := t.Preprocess(m, fullmd, name)
	// zlog.Info("ConvertToHTML:", m.Variables, input)
	params := blackfriday.HTMLRendererParameters{}
	params.Title = name
	params.Flags = blackfriday.CompletePage | blackfriday.HrefTargetBlank
	params.CSS = zrest.AppURLPrefix + "css/zcore/github-markdown.css"
	renderer := blackfriday.NewHTMLRenderer(params)
	output := blackfriday.Run([]byte(input),
		blackfriday.WithExtensions(extensions|blackfriday.AutoHeadingIDs),
		blackfriday.WithRenderer(renderer))
	_, err := w.Write([]byte(output))
	return err
}

// var linkReg = regexp.MustCompile(`\[[\w\s]+\]\(([\w/]+\.md)\)`)
var linkFileReg = regexp.MustCompile(`\[[\s\w\*\.\:]+\]\(([\w/]+\.md)\)`)
var linkInterReg = regexp.MustCompile(`\[[\w\s]+\]\((#[\w/]+)\)`)
var footReg = regexp.MustCompile(`\s*\[.+\]\:`)
var headerReg = regexp.MustCompile(`^#{1,6}\s*(.+)`)
var headerWithLink = regexp.MustCompile(`(.+)\s+\[(.+)\]\((.+)\)\s*(.*)`)

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

func (m *MarkdownConverter) Flatten() (string, error) {
	var out, footers string
	var contents []content

	// var topFileAnchors []chapterAnchor

	// str := `### ![open](open.png) Open prefs`
	// zstr.ReplaceAllCapturesFunc(headerReg, str, func(capture string, index int) string {
	// 	zlog.Info("Replace:", capture)
	// 	return "xxx"
	// })
	// return "", nil
	for _, chapter := range m.PartNames {
		if strings.HasSuffix(chapter, SharedPageSuffix) {
			continue
		}
		spath := zstr.Concat("/", m.Dir, chapter)
		str, err := zfile.ReadStringFromFileInFS(m.FileSystem, spath)
		if err != nil {
			zlog.Error(err, spath)
			continue
		}
		zstr.RangeStringLines(str, false, func(s string) bool {
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
	if m.TableOfContents {
		out = getTableOfContents(contents)
	}
	for _, chapter := range m.PartNames {
		if strings.HasSuffix(chapter, SharedPageSuffix) {
			continue
		}
		spath := zstr.Concat("/", m.Dir, chapter)
		str, err := zfile.ReadStringFromFileInFS(m.FileSystem, spath)
		if err != nil {
			zlog.Error(err, spath)
			continue
		}
		zstr.RangeStringLines(str, false, func(s string) bool {
			// if !atFooters && footReg.MatchString(s) {
			// 	atFooters = true
			// }
			// if atFooters {
			// 	footers += s + "\n"
			// 	return true
			// }
			snew := zstr.ReplaceAllCapturesFunc(linkInterReg, s, 0, func(capture string, index int) string {
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
			snew = zstr.ReplaceAllCapturesFunc(linkFileReg, snew, 0, func(capture string, index int) string {
				// if zhttp.HasURLScheme(capture) {
				// 	return capture
				// }
				// if !strings.Contains(capture, "#") {
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
			snew = zstr.ReplaceAllCapturesFunc(headerReg, snew, 0, func(capture string, index int) string {
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
				// var rest string

				id := headerToAnchorID(capture)
				anchor := anchorFromFileAndAnchor(chapter, id)
				anchorEscaped := fmt.Sprintf("{{`{#%s}`}}", anchor)
				parts := zstr.GetAllCaptures(headerWithLink, capture)
				if len(parts) < 3 {
					nstr := fmt.Sprint(capture, " ", anchorEscaped)
					// zlog.Info("Anchor:", anchorEscaped)
					return nstr
				}
				var postText string
				if len(parts) > 3 {
					postText = parts[3]
				}
				title := parts[1]
				preText := parts[0]
				nstr := fmt.Sprintf("%s *%s* %s %s", preText, title, postText, anchorEscaped)
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

func (m *MarkdownConverter) ServeAsHTML(w http.ResponseWriter, req *http.Request, spath string) {
	defer req.Body.Close()
	input, err := zfile.ReadStringFromFileInFS(m.FileSystem, spath)
	if zlog.OnError(err, spath) {
		return
	}
	zrest.AddCORSHeaders(w, req)
	err = m.ConvertToHTML(w, input)
	if err != nil {
		zrest.ReturnAndPrintError(w, req, http.StatusInternalServerError, err, "convert")
		return
	}
}

func outputValue(empty bool, k, v string) string {
	if !empty || v != "" {
		return v
	}
	return "<" + k + ">"
}

func MakeCURLMarkdownDescriptor(empty bool, title string, restURL, path, method string, headers, args map[string]string, body, resultPtr any, err error) string {
	var md string
	if empty {
		headers["Content-Type"] = "application/json"
	}
	md += title + "\n"
	md += "```\n"
	md += "curl -X " + method + " \\\n"
	for k, v := range headers {
		md += "  -H \"" + k + ": " + outputValue(empty, k, v) + "\" \\\n"
	}
	if restURL == "" {
		restURL = "<rest-url>"
	}
	surl := restURL + path
	if len(args) != 0 {
		surl += "?"
		if empty {
			nargs := map[string]string{}
			for k, v := range args {
				nargs[k] = outputValue(empty, k, v)
			}
			surl += zstr.ArgsToString(nargs, "&", "=", "")
		} else {
			surl += zstr.GetArgsAsURLParameters(args)
		}
	}
	md += `  "` + surl + "\"\n"
	if body != nil {
		if empty {
			md += "-D '\\\n"
			var str string
			// str := zfields.OutputJsonStructDescription(body, "")
			str = strings.Replace(str, "\n", " \\\n", -1)
			md += str
			md += "'\\\n"
		} else {
			bdata, _ := json.Marshal(body)
			md += "-D '" + string(bdata) + "'\n"
		}
	}
	md += "```\n"
	if err != nil {
		md += "gave error:\n"
		md += err.Error()
		return md
	}
	if resultPtr == nil {
		return md
	}

	md += "returning:\n"
	md += "```\n"
	if empty {
		//	md += zfields.OutputJsonStructDescription(resultPtr, "")
	} else {
		bdata, _ := json.MarshalIndent(resultPtr, "", "  ")
		md += string(bdata) + "\n"
	}
	md += "```\n"
	return md
}
