//go:build server

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

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	blackfriday "github.com/torlangballe/blackfridayV2"
	"github.com/torlangballe/zui/zkeyboard"
	"github.com/torlangballe/zutil/zdevice"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
)

// https://godoc.org/github.com/russross/blackfriday

// type OutputType string

const (
	// OutputPDF  OutputType = "pdf"
	// OutputMD   OutputType = "md"
	// OutputHTML OutputType = "html"

	SharedPageSuffix = ".shared.md"
)

type MarkdownConverter struct {
	PartNames       []string
	Dir             string
	Variables       zdict.Dict
	FileSystem      fs.FS
	TableOfContents bool
	AbsolutePrefix  string
	HeaderMD        string
}

func pdfGrabber(w io.Writer, url string) chromedp.Tasks {
	return chromedp.Tasks{
		emulation.SetUserAgentOverride("WebScraper 1.0"),
		chromedp.Navigate(url),
		chromedp.WaitVisible("body", chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			params := page.PrintToPDFParams{ // https://github.com/chromedp/cdproto/blob/master/page/page.go#L823
				DisplayHeaderFooter:     true,
				MarginLeft:              0.7,
				MarginTop:               1.0,
				MarginBottom:            0.5,
				HeaderTemplate:          `<div style="width: 100%; font-size:12px; height: 20px; text-align:center" class=title></div>`,
				FooterTemplate:          `<div style="width: 100%; font-size:14px; height: 20px; text-align:center" class=pageNumber></div>`,
				GenerateDocumentOutline: true,
			}
			buf, _, err := params.Do(ctx)
			if err != nil {
				return err
			}
			w.Write(buf)
			return nil
		}),
	}
}

func ConvertFromHTMLToPDF(w io.Writer, surl string) error {
	ctx, cancel := chromedp.NewContext(
		context.Background(),
		chromedp.WithLogf(log.Printf),
	)
	defer cancel()
	err := chromedp.Run(ctx, pdfGrabber(w, surl))
	if err != nil {
		return zlog.Error("run", err)
	}
	return nil
}

func (m *MarkdownConverter) ConvertToHTML(w io.Writer, name string) error {
	fullmd, err := m.Flatten()
	if err != nil {
		return zlog.Error("building doc", name, err)
	}
	return m.ConvertToHTMLFromString(w, fullmd, name)
}

func (m *MarkdownConverter) ConvertToHTMLFromString(w io.Writer, fullmd, name string) error {
	var extensions = blackfriday.NoIntraEmphasis | blackfriday.Tables | blackfriday.FencedCode |
		blackfriday.Autolink | blackfriday.Strikethrough | blackfriday.SpaceHeadings | blackfriday.HeadingIDs |
		blackfriday.BackslashLineBreak | blackfriday.DefinitionLists | blackfriday.HardLineBreak

	templater := &Templater{TeXFontSize: 14, DPI: 144}

	input := templater.Preprocess(m, m.HeaderMD+fullmd, name)
	params := blackfriday.HTMLRendererParameters{}
	params.Title = name
	params.Flags = blackfriday.CompletePage | blackfriday.HrefTargetBlank
	//!!		params.Flags |= blackfriday.TOC
	params.CSS = zrest.AppURLPrefix + "css/zcore/github-markdown.css"
	zlog.Info("MarkDown CSS:", params.CSS)
	renderer := blackfriday.NewHTMLRenderer(params)
	renderer.AbsolutePrefix = m.AbsolutePrefix
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
var headerReplacer = strings.NewReplacer(" ", "_", "#", "", ".", "_")

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

func getTableOfContents(contents []content) string {
	// zlog.Info("getTableOfContents1")
	table := "\\\n### Table of Contents\n\n\\\n"
	for _, c := range contents {
		// zlog.Info("getTableOfContents:", c.title)
		table += "**[" + c.title + "](#" + c.anchor + ")**\n"
	}
	return table + "\\\n\\\n"
}

func (m *MarkdownConverter) Flatten() (string, error) {
	var out, footers string
	var contents []content
	for _, chapter := range m.PartNames {
		if strings.HasSuffix(chapter, SharedPageSuffix) {
			continue
		}
		spath := zstr.Concat("/", m.Dir, chapter)
		str, err := zfile.ReadStringFromFileInFS(m.FileSystem, spath)
		if err != nil {
			zlog.Error(spath, err)
			continue
		}
		var hasHeader bool
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
				hasHeader = true
				contents = append(contents, c)
				// topFileAnchors = append(topFileAnchors, c.anchor})
				// zlog.Info("ANCH:", topFileAnchors[chapter], chapter, id)
				return false
			}
			return true
		})
		if !hasHeader {
			return "", zlog.Error("No header for chapter:", chapter)
		}
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
			zlog.Error(spath, err)
			continue
		}
		zstr.RangeStringLines(str, false, func(s string) bool {
			snew := zstr.ReplaceAllCapturesFunc(linkInterReg, s, 0, func(capture string, index int) string {
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
				for _, c := range contents {
					if c.chapter == capture {
						link := "#" + c.anchor
						return link
					}
				}
				return ""
			})
			snew = zstr.ReplaceAllCapturesFunc(headerReg, snew, 0, func(capture string, index int) string {
				if strings.HasPrefix(capture, "!") {
					return capture
				}
				id := headerToAnchorID(capture)
				anchor := anchorFromFileAndAnchor(chapter, id)
				anchorEscaped := fmt.Sprintf("{{`{#%s}`}}", anchor)
				parts := zstr.GetAllCaptures(headerWithLink, capture)
				if len(parts) < 3 {
					nstr := fmt.Sprint(capture, " ", anchorEscaped)
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
			return "", zlog.Error("read lines from base", err)
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
	query := req.URL.Query()
	debugMode := (query.Get("zdev") == "1")
	isRaw := (query.Get("raw") == "1")
	osType := zdevice.OSTypeFromUserAgentString(req.Header.Get("User-Agent"))
	zlog.Info("MarkdownConverter.ServeAsHTML:", req.URL, debugMode)
	m.SetBrowserSpecificDocKeyValues(osType, debugMode)

	_, _, stub, _ := zfile.Split(spath)
	zrest.AddCORSHeaders(w, req)
	if isRaw {
		_, err = w.Write([]byte(input))
		return
	} else {
		err = m.ConvertToHTMLFromString(w, input, stub)
	}
	if err != nil {
		zrest.ReturnAndPrintError(w, req, http.StatusInternalServerError, "convert", err)
		return
	}
}

func (m *MarkdownConverter) SetBrowserSpecificDocKeyValues(os zdevice.OSType, debugMode bool) {
	metaMod := zkeyboard.ModifierControl
	altName := zkeyboard.AltModifierConstName
	if os == zdevice.MacOSType {
		metaMod = zkeyboard.ModifierCommand
		altName = zkeyboard.OptionModifierConstName
	}
	m.Variables["ZMenuModifier"] = metaMod.HumanString()
	m.Variables["ZAltModifier"] = altName
	m.Variables["ZDebugOwnerMode"] = debugMode
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
