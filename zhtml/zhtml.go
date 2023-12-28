package zhtml

import (
	"io"
	"strings"

	"github.com/torlangballe/zutil/zstr"
	"golang.org/x/net/html"
)

func cleanText(str string) string {
	for {
		if zstr.Replace(&str, "\n\n", "\n") {
			continue
		}
		if zstr.Replace(&str, "\n ", "\n") {
			continue
		}
		if zstr.Replace(&str, " \n", "\n") {
			continue
		}
		break
	}
	return strings.TrimSpace(str)
}

func ExtractTextFromHTMLString(shtml string) (text string, err error) {
	r := strings.NewReader(shtml)
	d := html.NewTokenizer(r)
	// FIXME: wtf is this?
	for {
		// token type
		tokenType := d.Next()
		if tokenType == html.ErrorToken {
			text = strings.TrimSpace(text)
			err = d.Err()
			if err == io.EOF {
				err = nil
			}
			return cleanText(text), err
		}
		token := d.Token()
		switch tokenType {
		case html.StartTagToken: // <tag>
			// type Token struct {
			//     Type     TokenType
			//     DataAtom atom.Atom
			//     Data     string
			//     Attr     []Attribute
			// }
			//
			// type Attribute struct {
			//     Namespace, Key, Val string
			// }
		case html.TextToken:
			text += token.Data
			//			zlog.Info("token str:", token.Data)
		case html.EndTagToken: // </tag>
		case html.SelfClosingTagToken: // <tag/>
		}
	}
}

func StringIsHTML(str string) bool {
	return strings.HasPrefix(str, "<!DOCTYPE HTML ")
}
