//go:build !windows

package keystore

import "errors"

// dpapiAvailable reports whether DPAPI protection is supported on this
// platform. Only Windows builds enable it.
const dpapiAvailable = false

var errDPAPIUnsupported = errors.New("DPAPI is only available on Windows")

func dpapiProtect(string) (string, error) {
	return "", errDPAPIUnsupported
}

func dpapiUnprotect(string) (string, error) {
	return "", errDPAPIUnsupported
}
