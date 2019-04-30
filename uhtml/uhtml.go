package uhtml

import (
	"code.google.com/p/go.net/html"
	"strings"
)

func ExtractTextFromHTMLString(str string) (text string, err error) {
	r := strings.NewReader(str)
	d := html.NewTokenizer(r)
	// FIXME: wtf is this?
	for {
		// token type
		tokenType := d.Next()
		if tokenType == html.ErrorToken {
			text = strings.TrimSpace(text)
			err = d.Err()
			return
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
			//			fmt.Println("token str:", token.Data)
		case html.EndTagToken: // </tag>
		case html.SelfClosingTagToken: // <tag/>
		}
	}
}
