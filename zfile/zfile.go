package zfile

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/torlangballe/zutil/zstr"
)

var RootFolder = getRootFolder()

func getRootFolder() string {
	return ExpandTildeInFilepath("~/caproot/")
}

func CreateTempFile(name string) (file *os.File, filepath string, err error) {
	filepath = CreateTempFilePath(name)
	//	fmt.Println("filepath:", filepath)
	file, err = os.Create(filepath)
	if file == nil {
		fmt.Println("Error creating temporary template edit file", err, filepath)
	}
	return
}

func CreateTempFilePath(name string) string {
	now := time.Now()
	sdate := now.Format("2006-01-02")
	sfold := filepath.Join(os.TempDir(), sdate)
	err := os.MkdirAll(sfold, 0775|os.ModeDir)
	if err != nil {
		fmt.Println("zfile.CreateTempFilePath:", err)
		return ""
	}
	stime := now.Format("150405_999999")
	stemp := filepath.Join(sfold, SanitizeStringForFilePath(stime+"_"+name))
	return stemp
}

func FileExist(filepath string) bool {
	_, err := os.Stat(filepath)
	return err == nil
}

func SetModified(filepath string, t time.Time) error {
	err := os.Chtimes(filepath, t, t)
	return err
}

func FileNotExist(filepath string) bool { // not same as !DoesFileExist...
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		return true
	}
	return false
}

func IsFolder(filepath string) bool {
	stat, err := os.Stat(filepath)
	if err != nil {
		return false
	}
	return stat.IsDir()
}

func ReadStringFromFile(sfile string) (string, error) {
	bytes, err := ioutil.ReadFile(sfile)
	if err != nil {
		err = errors.Wrapf(err, "zfile.ReadFileToString: %v", sfile)
		//		fmt.Println("Error reading file:", sfile, err)
		return "", err
	}
	return string(bytes), nil
}

func WriteStringToFile(str, sfile string) (err error) {
	err = ioutil.WriteFile(sfile, []byte(str), 0644)
	if err != nil {
		//		fmt.Println("Error reading file:", sfile, err)
		return err
	}
	return
}

func ForAllFileLines(path string, f func(str string) bool) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	scanner := bufio.NewScanner(file)

	skip := false
	for scanner.Scan() {
		if !skip {
			ok := f(scanner.Text())
			if !ok {
				skip = true
			}
		}
	}
	return scanner.Err()
}

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

func Size(filepath string) int64 {
	stat, err := os.Stat(filepath)
	if err == nil {
		return stat.Size()
	}
	return -1
}

func Modified(filepath string) time.Time {
	stat, err := os.Stat(filepath)
	if err == nil {
		return stat.ModTime()
	}
	return time.Time{}
}

func CalcMD5(filePath string) (data []byte, err error) {
	file, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer file.Close()
	hash := md5.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return
	}
	data = hash.Sum(nil)[:16]
	return
}

func ReadFromURLToFilepath(surl, filePath string, maxBytes int64) (path string, err error) {
	if filePath == "" {
		var name string
		u, err := url.Parse(surl)
		if err != nil {
			_, name, _, ext := Split(surl)
			name = zstr.HeadUntil(name, "?") + ext
		} else {
			name = strings.Trim(u.Path, "/")
		}
		filePath = CreateTempFilePath(name)
	}
	response, err := http.Get(surl)
	if err != nil {
		fmt.Println("ReadFromUrlToFilepath error getting:", err, surl)
		return
	}
	defer response.Body.Close()

	//open a file for writing
	file, err := os.Create(filePath)
	if err != nil {
		fmt.Println("ReadFromUrlToFilepath error creating file:", err, filePath)
		return
	}
	if maxBytes != 0 {
		var size int64
		buf := make([]byte, maxBytes)
		for size < maxBytes {
			var n int
			n, err = response.Body.Read(buf)
			if err != nil && err != io.EOF {
				fmt.Println("Error reading from body:", err)
				return
			}
			_, err = file.Write(buf[:n])
			if err != nil {
				fmt.Println("Error writing from body:", err)
				return
			}
			size += int64(n)
			if n == 0 {
				break
			}
		}
	} else {
		// Use io.Copy to just dump the response body to the file. This supports huge files
		_, err = io.Copy(file, response.Body)
		if err != nil {
			fmt.Println("ReadFromUrlToFilepath error copying to file:", err, filePath)
			return
		}
	}
	file.Close()
	path = filePath
	return
}

func Walk(folder, wildcard string, got func(fpath string, info os.FileInfo) error) {
	filepath.Walk(folder, func(fpath string, info os.FileInfo, err error) error {
		if err == nil {
			if wildcard != "" {
				_, name := filepath.Split(fpath)
				matched, _ := filepath.Match(wildcard, name)
				if !matched {
					return nil
				}
				e := got(fpath, info)
				if e != nil {
					return e
				}
			}
		}
		return nil
	})
}

func RemoveOldFilesFromFolder(folder, wildcard string, olderThan time.Duration) {
	Walk(folder, wildcard, func(fpath string, info os.FileInfo) error {
		if time.Since(info.ModTime()) > olderThan {
			os.Remove(fpath)
		}
		return nil
	})
}

func RemoveContents(dir string) error {
	d, err := os.Open(dir)
	// fmt.Println("RemoveContents:", dir, err)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(dir, name))
		if err != nil {
			return err
		}
	}
	return nil
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
