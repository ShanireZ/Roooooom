package i18n

import (
	"syscall"
	"unsafe"
)

var kernel32 = syscall.NewLazyDLL("kernel32.dll")

// OS returns the Windows user interface language, not the regional format locale.
func OS() (Locale, bool) {
	lang, _, _ := kernel32.NewProc("GetUserDefaultUILanguage").Call()
	if lang == 0 {
		return "", false
	}
	buf := make([]uint16, 85)
	n, _, _ := kernel32.NewProc("LCIDToLocaleName").Call(lang, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), 0)
	if n == 0 {
		return "", false
	}
	return Parse(syscall.UTF16ToString(buf))
}
