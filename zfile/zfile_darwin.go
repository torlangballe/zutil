package zfile

import "golang.org/x/sys/unix"

func CloneFile(dest, source string) error {
	return unix.Clonefile(source, dest, 0)
}
