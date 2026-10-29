//go:build !windows

package i18n

import (
	"os"
	"strings"
)

// OS returns the POSIX message locale from LC_ALL, LC_MESSAGES or LANG.
func OS() (Locale, bool) {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		value, _, _ := strings.Cut(os.Getenv(name), ".")
		if value == "" || value == "C" || value == "POSIX" {
			continue
		}
		return Parse(value)
	}
	return "", false
}
