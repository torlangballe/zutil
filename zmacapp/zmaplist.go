//go:build zui && catalyst

package zmacapp

import (
	"strings"

	"github.com/torlangballe/zutil/zfile"
)

const plistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key>
	<string>{{.AppName}}</string>
	<key>RunAtLoad</key>
	<true/>
	<key>CFBundleIdentifier</key>
	<string>{{.BundleIdentifier}}</string>
	{{.CommandLines}}
	<key>LSUIElement</key>
	<true/>
	<key>CFBundlePackageType</key>
	<string>APPL</string>
</dict>
</plist>
`

// <key>CFBundleIconFile</key>
// <string>icon.icns</string>

func WriteAppPList(topath, binaryName, bundleIdentifier string, args []string) error {
	plist := strings.Replace(plistTemplate, "{{.AppName}}", binaryName, -1)
	plist = strings.Replace(plist, "{{.BundleIdentifier}}", bundleIdentifier, -1)
	var sargs string
	if len(args) != 0 {
		sargs += "<key>ProgramArguments</key>\n<array>\n"
		for _, a := range args {
			sargs += "  <string>" + a + "</string>\n"
		}
		sargs += "</array>\n"
	}
	plist = strings.Replace(plist, "{{.CommandLines}}", sargs, -1)
	return zfile.WriteStringToFile(plist, topath)
}

// <key>StartInterval</key>
// <integer>60</integer>
