//go:build !windows

package cliutil

import "os"

func createPrivateTempFile(dir, base string) (*os.File, error) {
	return os.CreateTemp(dir, "."+base+".*.tmp")
}
