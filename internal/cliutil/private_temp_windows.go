//go:build windows

package cliutil

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Set the DACL at creation, not afterwards: an inherited reader must never
// obtain a handle to the file before we write private bytes into it.
func createPrivateTempFile(dir, base string) (*os.File, error) {
	sid, err := currentUserSID()
	if err != nil {
		return nil, err
	}
	sd, err := windows.SecurityDescriptorFromString(fmt.Sprintf("O:%sD:P(A;;FA;;;%s)", sid, sid))
	if err != nil {
		return nil, err
	}
	var suffix [16]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "."+base+"."+hex.EncodeToString(suffix[:])+".tmp")
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	attributes := windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		&attributes, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(handle), path), nil
}
