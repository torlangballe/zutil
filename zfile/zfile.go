package zfile

import (
	"bufio"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
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
	"github.com/torlangballe/zutil/ustr"
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
	stime := time.Now().Format(time.RFC3339Nano)
	sfold := filepath.Join(os.TempDir(), stime)
	err := os.MkdirAll(sfold, 0775|os.ModeDir)
	if err != nil {
		fmt.Println("ufile.CreateTempFilePath:", err)
		return ""
	}
	stemp := filepath.Join(sfold, SanitizeStringForFilePath(name))
	return stemp
}

func DoesFileExist(filepath string) bool {
	_, err := os.Stat(filepath)
	return err == nil
}

func SetModified(filepath string, t time.Time) error {
	err := os.Chtimes(filepath, t, t)
	return err
}

func DoesFileNotExist(filepath string) bool { // not same as !DoesFileExist...
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

func ReadFileToString(sfile string) (string, error) {
	bytes, err := ioutil.ReadFile(sfile)
	if err != nil {
		err = errors.Wrapf(err, "ufile.ReadFileToString: %v", sfile)
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

func Split(spath string) (dir, name, ext string) {
	dir, name = path.Split(spath)
	ext = path.Ext(name)
	name = strings.TrimSuffix(name, ext)

	return
}

func SanitizeStringForFilePath(s string) string {
	s = url.QueryEscape(s)
	s = ustr.FileEscapeReplacer.Replace(s)

	return s
}

func CreateSanitizedShortNameWithHash(name string) string {
	hash := ustr.HashTo64Hex(name)
	name = ustr.Head(name, 100)
	name = ustr.ReplaceSpaces(name, '_')
	name = SanitizeStringForFilePath(name)
	name = name + "#" + hash

	return name
}

func ExpandTildeInFilepath(path string) string {
	if runtime.GOOS == "js" {
		return ""
	}
	usr, err := user.Current()
	if err == nil {
		dir := usr.HomeDir
		return strings.Replace(path, "~", dir, 1)
	}
	return ""
}

func GetSize(filepath string) (size int64, err error) {
	stat, err := os.Stat(filepath)
	if err == nil {
		size = stat.Size()
	}
	return
}

func calcMD5InBytes(filePath string) (data []byte, err error) {
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

func CalcMD5Hex(filePath string) (string, error) {
	data, err := calcMD5InBytes(filePath)
	if err != nil {
		return "", err
	}
	md5s := hex.EncodeToString(data)
	return md5s, nil
}

func CalcMD5Base64(filePath string) (string, error) {
	data, err := calcMD5InBytes(filePath)
	if err != nil {
		return "", err
	}
	md5s := base64.StdEncoding.EncodeToString(data)
	return md5s, nil
}

func ReadFromUrlToFilepath(surl, filePath string, maxBytes int64) (path string, err error) {
	if filePath == "" {
		var name string
		u, err := url.Parse(surl)
		if err != nil {
			_, name, ext := Split(surl)
			name = ustr.HeadUntilString(name, "?") + ext
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
