package outbound

import (
	"net/url"
	"regexp"
)

var credentialPattern = regexp.MustCompile(`(?i)(Bearer\s+)[^\s"<>]+|(?:github_pat_|gh[pousr]_)[a-zA-Z0-9_]+|([?&](?:access_token|token|sign|secret|key)=)[^\s&#"<>]+`)

var urlPattern = regexp.MustCompile(`https?://[^\s"<>]+`)

func Redact(s string) string {
	s = urlPattern.ReplaceAllStringFunc(s, func(raw string) string {
		u, e := url.Parse(raw)
		if e != nil {
			return "[redacted-url]"
		}
		sensitive := u.User != nil || u.Hostname() == "oapi.dingtalk.com"
		for key := range u.Query() {
			switch key {
			case "access_token", "token", "sign", "secret", "key":
				sensitive = true
			}
		}
		if sensitive {
			u.User = nil
			u.RawQuery = ""
			u.Fragment = ""
			return u.String() + "?[redacted]"
		}
		return raw
	})
	return credentialPattern.ReplaceAllString(s, "[redacted]")
}
