// +build !darwin

package zfile

import "errors"

func CloneFile(dest, source string) error {
	return errors.New("Not implemented!")
}
