package zmarkdown

import (
	"gopkg.in/russross/blackfriday.v2"
	// import "github.com/mandolyte/mdtopdf"
)

// https://godoc.org/github.com/russross/blackfriday

func Convert(input, title string) (string, error) {
	params := blackfriday.HTMLRendererParameters{}
	params.Title = title
	params.Flags = blackfriday.CompletePage | blackfriday.HrefTargetBlank
	params.CSS = "http://localhost/css/github-markdown.css"

	nh := blackfriday.NewHTMLRenderer(params)
	output := blackfriday.Run([]byte(input),
		blackfriday.WithExtensions(blackfriday.CommonExtensions|blackfriday.HardLineBreak),
		blackfriday.WithRenderer(nh))
	return string(output), nil
}
