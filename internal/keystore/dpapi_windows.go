//go:build windows

package keystore

import (
	"encoding/base64"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

// dpapiAvailable reports whether DPAPI protection is supported on this
// platform. Only Windows builds enable it.
const dpapiAvailable = true

// dataBlob mirrors the Windows DATA_BLOB structure.
type dataBlob struct {
	cbData uint32
	pbData *byte
}

var (
	crypt32            = syscall.NewLazyDLL("crypt32.dll")
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procCryptProtect   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotect = crypt32.NewProc("CryptUnprotectData")
	procLocalFree      = kernel32.NewProc("LocalFree")
)

// dpapiProtect encrypts plain with CryptProtectData in CurrentUser scope and
// returns the base64-encoded blob. This is the Go equivalent of the upstream
// PowerShell call-out and produces the same blob format.
func dpapiProtect(plain string) (string, error) {
	protected, err := dpapiCall(procCryptProtect, []byte(plain))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(protected), nil
}

// dpapiUnprotect decrypts a base64-encoded CryptProtectData blob.
func dpapiUnprotect(encoded string) (string, error) {
	blob, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return "", err
	}
	plain, err := dpapiCall(procCryptUnprotect, blob)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// dpapiCall invokes CryptProtectData or CryptUnprotectData with null
// description, entropy, and prompt structs — the same arguments .NET's
// ProtectedData passes for CurrentUser scope. The output blob is released
// with LocalFree.
func dpapiCall(proc *syscall.LazyProc, input []byte) ([]byte, error) {
	var inputBlob dataBlob
	if len(input) > 0 {
		inputBlob = dataBlob{cbData: uint32(len(input)), pbData: &input[0]}
	}
	var outputBlob dataBlob
	result, _, callErr := proc.Call(
		uintptr(unsafe.Pointer(&inputBlob)),
		0,
		0,
		0,
		0,
		0,
		uintptr(unsafe.Pointer(&outputBlob)),
	)
	runtime.KeepAlive(input)
	if result == 0 {
		if callErr != syscall.Errno(0) {
			return nil, fmt.Errorf("%s failed: %w", proc.Name, callErr)
		}
		return nil, fmt.Errorf("%s failed", proc.Name)
	}
	output := make([]byte, outputBlob.cbData)
	if outputBlob.cbData > 0 && outputBlob.pbData != nil {
		copy(output, unsafe.Slice(outputBlob.pbData, outputBlob.cbData))
	}
	_, _, _ = procLocalFree.Call(uintptr(unsafe.Pointer(outputBlob.pbData)))
	return output, nil
}
