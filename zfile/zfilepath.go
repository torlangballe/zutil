package zfile

import (
	"net/url"
	"os/user"
	"path"
	"runtime"
	"strings"

	"github.com/torlangballe/zutil/zstr"
)

func RemovedExtension(spath string) string {
	name := strings.TrimSuffix(spath, path.Ext(spath))
	return name
}

func Split(spath string) (dir, name, stub, ext string) {
	dir, name = path.Split(spath)
	ext = path.Ext(name)
	stub = strings.TrimSuffix(name, ext)

	return
}

func SanitizeStringForFilePath(s string) string {
	s = url.QueryEscape(s)
	s = zstr.FileEscapeReplacer.Replace(s)

	return s
}

func CreateSanitizedShortNameWithHash(name string) string {
	hash := zstr.HashTo64Hex(name)
	name = zstr.Head(name, 100)
	name = zstr.ReplaceSpaces(name, '_')
	name = SanitizeStringForFilePath(name)
	name = name + "#" + hash

	return name
}

func ExpandTildeInFilepath(path string) string {
	if runtime.GOOS == "js" {
		return path
	}
	usr, err := user.Current()
	if err == nil {
		dir := usr.HomeDir
		return strings.Replace(path, "~", dir, 1)
	}
	return ""
}

func ReplaceHomeDirPrefixWithTilde(path string) string {
	var rest string
	if runtime.GOOS == "js" {
		return path
	}
	usr, err := user.Current()
	if err != nil {
		return path
	}
	dir := usr.HomeDir + "/"
	if zstr.HasPrefix(path, dir, &rest) {
		return "~/" + rest
	}
	return path
}

func MakePathRelativeTo(path, rel string) string {
	origPath := path
	path = strings.TrimLeft(path, "/")
	rel = strings.TrimLeft(rel, "/")
	// fmt.Println("MakePathRelativeTo1:", path, rel)
	for {
		p := zstr.HeadUntil(path, "/")
		r := zstr.HeadUntil(rel, "/")
		if p != r || p == "" {
			break
		}
		l := len(p)
		path = zstr.Body(path, l+1, -1)
		rel = zstr.Body(rel, l+1, -1)
	}
	// fmt.Println("MakePathRelativeTo:", path, rel)
	count := strings.Count(rel, "/")
	if count != 0 {
		count++
	}
	str := strings.Repeat("../", count) + path
	if count > 2 || len(str) > len(origPath) {
		return ReplaceHomeDirPrefixWithTilde(origPath)
	}
	return str
}
