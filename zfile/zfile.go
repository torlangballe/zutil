//go:build !js

package zfile

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztimer"
	"github.com/torlangballe/zutil/zwords"
)

type WalkOptions int

const (
	WalkOptionsNone     WalkOptions = 0
	WalkOptionRecursive WalkOptions = 1 << iota
	WalkOptionGiveFolders
	WalkOptionGiveHidden
	WalkOptionGiveNameOnly
	WalkOptionRelativePath
)

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
	sdate := now.Format("2006-01-02/")
	sfold := JoinPathParts(os.TempDir(), sdate)
	err := os.MkdirAll(sfold, 0775|os.ModeDir)
	if err != nil {
		fmt.Println("zfile.CreateTempFolder:", err)
		return ""
	}
	stime := now.Format("150405.999999")
	stime = strings.Replace(stime, ".", "_", -1)
	stemp := JoinPathParts(sfold, SanitizeStringForFilePath(stime+"_"+name))
	return stemp
}

func Exists(fpath string) bool {
	_, err := os.Stat(fpath)
	return err == nil
}

func NotExists(fpath string) bool { // not same as !DoesFileExist...
	if _, err := os.Stat(fpath); os.IsNotExist(err) {
		return true
	}
	return false
}

func MakeDirAllIfNotExists(dir string) error {
	err := os.MkdirAll(dir, os.ModeDir|0755)
	if err == nil || os.IsExist(err) {
		return nil
	}
	return err
}

func SetModified(fpath string, t time.Time) error {
	err := os.Chtimes(fpath, t, t)
	return err
}

func IsFolder(fpath string) bool {
	stat, err := os.Stat(fpath)
	if err != nil {
		return false
	}
	return stat.IsDir()
}

func ReadStringFromFile(sfile string) (string, error) {
	bytes, err := os.ReadFile(sfile)
	if err != nil {
		err = fmt.Errorf("zfile.ReadFileToString: %w: %s", err, sfile)
		//		fmt.Println("Error reading file:", sfile, err)
		return "", err
	}
	return string(bytes), nil
}

func WriteBytesToFile(data []byte, sfile string) error {
	return WriteToFileAtomically(sfile, func(file io.Writer) error {
		_, err := file.Write(data)
		return err
	})
}

func WriteStringToFile(str, sfile string) error {
	return WriteBytesToFile([]byte(str), sfile)
}

func SetOwnerAndMainGroup(fpath, owner string) error {
	// zlog.Info("SetOwnerAndMainGroup", fpath, owner)
	u, err := user.Lookup(owner)
	if err != nil {
		return err
	}
	uid, _ := strconv.Atoi(u.Uid)
	if err != nil {
		return err
	}
	gid, _ := strconv.Atoi(u.Gid)
	if err != nil {
		return err
	}
	err = os.Chown(fpath, uid, gid)
	// zlog.Info("SetOwnerAndMainGroup2", fpath, owner, err)
	return err
}

func ForAllFileLines(path string, skipEmpty bool, line func(str string) bool) error {
	str, err := ReadStringFromFile(path)
	if err != nil {
		return err
	}
	//TODO: Don't read file to memory
	zstr.RangeStringLines(str, skipEmpty, line)
	return nil
}

func Size(fpath string) int64 {
	stat, err := os.Stat(fpath)
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

func CalcMD5(filepath string) (data []byte, err error) {
	file, err := os.Open(filepath)
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

func CopyFile(dest, source string) (err error) {
	asource, err := filepath.Abs(source)
	if zlog.OnError(err, source) {
		return err
	}
	adest, err := filepath.Abs(dest)
	if zlog.OnError(err, dest) {
		return err
	}
	if asource == adest {
		zlog.Info("zfile.CopyFile: source == dest, skipping.", source, dest)
		return nil
	}
	err = CloneFile(dest, source)
	if err == nil {
		return
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

// ReadFromURLToFilepath http.Get's the file at surl, and stores it in a file at filepath.
// If maxBytes != 0, it only downloads that many bytes.
// If filepath == "", it creates a temporary file using name from surl.
// The stored file is returned in path, or err if error.
func ReadFromURLToFilepath(surl, filepath string, maxBytes int64) (path string, err error) {
	if filepath == "" {
		var name string
		u, err := url.Parse(surl)
		if err != nil {
			_, name, _, ext := Split(surl)
			name = zstr.HeadUntil(name, "?") + ext
		} else {
			name = strings.Trim(u.Path, "/")
		}
		filepath = CreateTempFilePath(name)
	}
	response, err := http.Get(surl)
	if err != nil {
		fmt.Println("ReadFromUrlToFilepath error getting:", err, surl)
		return
	}
	defer response.Body.Close()

	//open a file for writing
	file, err := os.Create(filepath)
	if err != nil {
		fmt.Println("ReadFromUrlToFilepath error creating file:", err, filepath)
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
			fmt.Println("ReadFromUrlToFilepath error copying to file:", err, filepath)
			return
		}
	}
	file.Close()
	path = filepath
	return
}

// Walk walks though the contents of folder, calling got on each, matching names with tab-separated wildcards (if any).
// folders are entered without wildcard matching.
// info.IsDir can be checked to see if the content is a folder, and path.SkipDir / path.SkipAll can be returned
// to abort a sub-folder or all. Any other error returned from got stops all and returns that error.
func Walk(folder, wildcards string, opts WalkOptions, got func(fpath string, info os.FileInfo) error) error {
	var wcards []string
	if wildcards != "" {
		wcards = strings.Split(wildcards, "\t")
	}
	afterFolderIndex := len(folder)
	if zstr.FirstRune(folder) != '/' {
		afterFolderIndex++
	}
	return filepath.Walk(folder, func(fpath string, info os.FileInfo, err error) error {
		// fmt.Println("zFile walk:", fpath, len(wcards), err)
		if err != nil {
			return err
		}
		if info.IsDir() {
			if fpath == folder {
				return nil
			}
		}
		_, name := filepath.Split(fpath)
		// zlog.Info("zWalk:", fpath)
		if opts&WalkOptionGiveHidden == 0 && strings.HasPrefix(name, ".") {
			// zlog.Info("isHidden?:", name)
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rpath := fpath
		if opts&WalkOptionRelativePath != 0 {
			if len(rpath) >= afterFolderIndex {
				rpath = rpath[afterFolderIndex:]
			} else {
				zlog.Error("fpath not bigger than folder:", fpath, folder)
			}
		} else if opts&WalkOptionGiveNameOnly != 0 {
			rpath = name
		}
		if info.IsDir() {
			if opts&WalkOptionGiveFolders != 0 {
				e := got(rpath, info)
				// zlog.Info("zWalk dir:", rpath, opts&WalkOptionGiveFolders, e)
				if e != nil {
					return e
				}
			}
			if opts&WalkOptionRecursive != 0 {
				return nil
			}
			return filepath.SkipDir
		}
		matched := true
		if len(wcards) > 0 {
			matched = false
			for _, w := range wcards {
				m, _ := filepath.Match(w, name)
				// zlog.Info("zWalk?:", w, name, m)
				if m {
					matched = true
					break
				}
			}
		}
		if !matched {
			return nil
		}
		return got(rpath, info)
	})
}

// GetFilesFromPath returns a list of names of files inside path.
func GetFilesFromPath(path, wildcards string, opts WalkOptions) (files []string, err error) {
	Walk(path, wildcards, opts, func(fpath string, info os.FileInfo) error {
		files = append(files, fpath)
		return nil
	})
	return files, nil
}

func RemoveOldFilesFromFolder(folder, wildcard string, opt WalkOptions, olderThan time.Duration) {
	Walk(folder, wildcard, opt, func(fpath string, info os.FileInfo) error {
		if time.Since(info.ModTime()) > olderThan {
			os.Remove(fpath)
		}
		return nil
	})
}

func RemoveAllQuicklyWithRename(dir string) error {
	dir = ExpandTildeInFilepath(dir)
	dir = strings.TrimRight(dir, "/")
	newName := dir + "-temp" + zstr.GenerateRandomHexBytes(12)
	err := os.Rename(dir, newName)
	if err != nil {
		return zlog.NewError(err, dir, newName)
	}
	go func() {
		os.RemoveAll(newName)
	}()
	return nil
}

func RemoveFolderWithContents(dir string) error {
	err := RemoveContents(dir)
	if err != nil {
		return err
	}
	return os.Remove(dir)
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
		err = os.RemoveAll(JoinPathParts(dir, name))
		if err != nil {
			return err
		}
	}
	return nil
}

func AppendToFile(fpath, str string) error {
	f, err := os.OpenFile(fpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(str)
	f.Close()
	return err
}

func handleErr(err error, why, path string, file *os.File, close bool) error {
	var ferr error
	if close {
		ferr = file.Close()
	}
	rerr := os.Remove(path)
	if ferr != nil || rerr != nil {
		zlog.Error("{nolog}WriteToFileAtomically call write func", ferr, rerr, err)
	}
	return err
}

// WriteToFileAtomically  opens a temporary file in same directory as fpath, calls write with its file,
// closes it, and renames it to fpath
func WriteToFileAtomically(fpath string, write func(file io.Writer) error) error {
	tempPath := fpath + fmt.Sprintf("_%x_ztemp", rand.Int31())
	file, err := os.Create(tempPath)
	if err != nil {
		zlog.Error("create", fpath, err)
		return err
	}
	err = write(file)
	if err != nil {
		return handleErr(err, "write", tempPath, file, true)
	}
	err = file.Close()
	if err != nil {
		return handleErr(err, "close", tempPath, file, false)
	}
	// zlog.Error("WriteToFileAtomically:", tempPath, "->", fpath, Exists(tempPath), Size(tempPath))
	err = os.Rename(tempPath, fpath)
	if err != nil {
		return handleErr(err, "rename", tempPath, file, false)
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
	file.Seek(diff, io.SeekStart)
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

// PeriodicFileBackup checks if *filepath* is larger than maxMB megabytes
// every *checkHours*. If so, the file is moved to a file in the  same directory
// with a suffix before extension. "path/file_suffix.log".
func PeriodicFileBackup(filepath, suffixForOld string, maxMB int) {
	ztimer.RepeatNow(60*10, func() bool {
		over := Size(filepath) >= int64(maxMB*1024*1024)
		zlog.Info("🟩PeriodicFileBackup", filepath, true, zwords.GetStorageSizeString(Size(filepath), "", 1))
		if over {
			dir, _, stub, ext := Split(filepath)
			newPath := dir + stub + suffixForOld + ext
			err := os.Remove(newPath)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				fmt.Println(err, "remove old", filepath, newPath)
				return true
			}
			err = CopyFile(newPath, filepath)
			if err != nil {
				fmt.Println(err, "copy to file backup", filepath, newPath)
				return true
			}
			err = os.Truncate(filepath, 0)
			if err != nil {
				fmt.Println(err, "remove old file backup", filepath, newPath)
				return true
			}
		}
		return true
	})
}

func DeleteOldInSubFolders(dir string, sleep time.Duration, before time.Time, deleteRatio float32, progress func(p float32, count, total int)) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	var total, count int
	var t time.Time
	nlen := float32(len(names))
	for i, name := range names {
		if deleteRatio < 1 {
			if rand.Float32() > deleteRatio {
				// zlog.Info("Skipping folder for cache:", dir, i, onlyRandomOneOf)
				continue
			}
		}
		fold := zstr.Concat("/", dir, name)
		err = filepath.Walk(fold, func(fpath string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if info.ModTime().Sub(before) < 0 {
				count++
				os.Remove(fpath)
			}
			total++
			return nil
		})
		if err != nil {
			zlog.Error("walking subdir", fold, err)
		}
		if progress != nil && time.Since(t) > time.Second*10 {
			t = time.Now()
			progress(float32(i)/nlen, count, total)
		}
		if sleep != 0 {
			time.Sleep(sleep)
		}
	}
	if progress != nil {
		progress(1, count, total)
	}
	return nil
}

// WorkingDirPathToAbsolute prefixes wpath with working dir.
// If wpath is absolute, it returns it as-is.
func WorkingDirPathToAbsolute(wpath string) string {
	if strings.HasPrefix(wpath, "/") {
		return wpath
	}
	wd, _ := os.Getwd()
	return zstr.Concat("/", wd, wpath)
}

// merge fs.FS interface inspiration:
// https://github.com/yalue/merged_fs/blob/master/merged_fs.go
