package zfile

import (
	"bufio"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zstr"
)

var RootFolder = getRootFolder()

func getRootFolder() string {
	return ExpandTildeInFilepath("~/caproot/")
}

func CreateTempFile(name string) (file *os.File, fpath string, err error) {
	fpath = CreateTempFilePath(name)
	//	fmt.Println("fpath:", fpath)
	file, err = os.Create(fpath)
	if file == nil {
		fmt.Println("Error creating temporary template edit file", err, fpath)
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

func Exists(fpath string) bool {
	_, err := os.Stat(fpath)
	return err == nil
}

func SetModified(fpath string, t time.Time) error {
	err := os.Chtimes(fpath, t, t)
	return err
}

func NotExist(fpath string) bool { // not same as !DoesFileExist...
	if _, err := os.Stat(fpath); os.IsNotExist(err) {
		return true
	}
	return false
}

func IsFolder(fpath string) bool {
	stat, err := os.Stat(fpath)
	if err != nil {
		return false
	}
	return stat.IsDir()
}

func ReadStringFromFile(sfile string) (string, error) {
	bytes, err := ioutil.ReadFile(sfile)
	if err != nil {
		err = fmt.Errorf("zfile.ReadFileToString: %w: %s", err, sfile)
		//		fmt.Println("Error reading file:", sfile, err)
		return "", err
	}
	return string(bytes), nil
}

func WriteStringToFile(str, sfile string) error {
	return WriteToFileAtomically(sfile, func(file io.Writer) error {
		_, err := file.Write([]byte(str))
		return err
	})
}

func ForAllFileLines(path string, f func(str string) bool) error {
	wd, _ := os.Getwd()
	file, err := os.Open(path)
	// fmt.Println("ForAllFileLines:", wd, path, err)
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

func Size(fpath string) int64 {
	stat, err := os.Stat(fpath)
	if err == nil {
		return stat.Size()
	}
	return -1
}

func Modified(fpath string) time.Time {
	stat, err := os.Stat(fpath)
	if err == nil {
		return stat.ModTime()
	}
	return time.Time{}
}

func CalcMD5(fpath string) (data []byte, err error) {
	file, err := os.Open(fpath)
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

func ReadFromURLToFilepath(surl, fpath string, maxBytes int64) (path string, err error) {
	if fpath == "" {
		var name string
		u, err := url.Parse(surl)
		if err != nil {
			_, name, _, ext := Split(surl)
			name = zstr.HeadUntil(name, "?") + ext
		} else {
			name = strings.Trim(u.Path, "/")
		}
		fpath = CreateTempFilePath(name)
	}
	response, err := http.Get(surl)
	if err != nil {
		fmt.Println("ReadFromUrlToFilepath error getting:", err, surl)
		return
	}
	defer response.Body.Close()

	//open a file for writing
	file, err := os.Create(fpath)
	if err != nil {
		fmt.Println("ReadFromUrlToFilepath error creating file:", err, fpath)
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
			fmt.Println("ReadFromUrlToFilepath error copying to file:", err, fpath)
			return
		}
	}
	file.Close()
	path = fpath
	return
}

func Walk(folder, wildcards string, got func(fpath string, info os.FileInfo) error) {
	wcards := strings.Split(wildcards, "\t")
	filepath.Walk(folder, func(fpath string, info os.FileInfo, err error) error {
		if err == nil {
			matched := true
			if len(wcards) > 0 {
				_, name := filepath.Split(fpath)
				matched = false
				for _, w := range wcards {
					m, _ := filepath.Match(w, name)
					if m {
						matched = true
						break
					}
				}
			}
			if matched {
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
	dir = ExpandTildeInFilepath(dir)
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

// WriteToFileAtomically  opens a temporary file in same directory as fpath, calls write with it's file,
// closes it, and renames it to fpath
func WriteToFileAtomically(fpath string, write func(file io.Writer) error) error {
	tempPath := fpath + fmt.Sprintf("_%x_ztemp", rand.Int31())
	file, err := os.Create(tempPath)
	if err != nil {
		return nil
	}
	defer file.Close()
	err = write(file)
	if err != nil {
		os.Remove(tempPath)
		fmt.Println(err, "{nolog}WriteToFileAtomically call write func")
		return err
	}
	err = os.Rename(tempPath, fpath)
	if err != nil {
		os.Remove(tempPath)
		fmt.Println("{nolog}WriteToFileAtomically rename", err, tempPath, fpath)
		return err
	}
	return nil
}

// if a file is > maxBytes, TruncateFile removes bytes from start or end to make it maxBytes*reduce large.
// This method is not atomical, more bytes can be added to fpath while it is working, and these will be lost,
// so a mutex or something should be used for appending to fpath if possible.
func TruncateFile(fpath string, maxBytes int64, reduce float64, fromEnd bool) error {
	if reduce >= 1 {
		panic("TruncateFile: reduce must be less that 1")
	}
	if fromEnd {
		panic("not implemented, though easy case")
	}
	size := Size(fpath)
	if size == -1 {
		return errors.New("zfile.TruncateFile: bad size  for:  " + fpath)
	}
	if size-maxBytes <= 0 {
		return nil
	}
	diff := size - int64(float64(maxBytes)*reduce)
	file, err := os.Open(fpath)
	if err != nil {
		return err
	}
	file.Seek(diff, os.SEEK_SET)
	err = WriteToFileAtomically(fpath, func(writeTo io.Writer) error {
		n, cerr := io.Copy(writeTo, file)
		fmt.Println("{nolog}TruncateFile write:", n, cerr)
		return cerr
	})
	file.Close()
	return err
}

// ReadLastLine reads a file from end, until it encounters ascii 10/13, consuming them too.
// *startpos* is where it started reading at.
// *newpos* is where it ended.
// if pos is not zero, it starts at pos. zero means start from end.
func ReadLastLine(fpath string, pos int64) (line string, startpos, newpos int64, err error) {
	file, err := os.Open(fpath)
	if err != nil {
		return "", 0, 0, err
	}
	defer file.Close()

	stat, _ := file.Stat()
	filesize := stat.Size()
	if pos == 0 {
		pos = filesize
		startpos = filesize
	} else {
		startpos = pos
	}
	found := false
	first := true
	for {
		pos--
		file.Seek(pos, io.SeekStart)
		char := make([]byte, 1)
		file.Read(char)
		if char[0] == 10 || char[0] == 13 { // stop if we find a line
			if first {
				continue
			}
			found = true
		} else {
			if found {
				pos++
				break
			}
			first = false
		}
		if !found {
			line = string(char) + line
		}
		if pos == 0 {
			break
		}
	}
	newpos = pos

	return
}
