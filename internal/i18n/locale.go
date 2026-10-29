// Package i18n resolves the owner-visible language for text the service renders
// itself: activation pages, tray labels and outbound notifications.
package i18n

import (
	"strconv"
	"strings"
)

type Locale string

const (
	EN   Locale = "en"
	ZHCN Locale = "zh-CN"
	ZHTW Locale = "zh-TW"
	JA   Locale = "ja"
)

// Parse maps a BCP 47 (or POSIX-style) tag to one of the four product locales.
func Parse(tag string) (Locale, bool) {
	parts := strings.Split(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(tag)), "_", "-"), "-")
	switch parts[0] {
	case "en":
		return EN, true
	case "ja":
		return JA, true
	case "zh":
		traditional := false
		for _, p := range parts[1:] {
			switch p {
			case "hans":
				return ZHCN, true
			case "hant":
				return ZHTW, true
			case "tw", "hk", "mo":
				traditional = true
			}
		}
		if traditional {
			return ZHTW, true
		}
		return ZHCN, true
	}
	return "", false
}

// FromAcceptLanguage picks the supported locale with the highest q-value; ties
// keep header order. Wildcards and q=0 entries never select a locale.
func FromAcceptLanguage(header string) (Locale, bool) {
	best, bestQ := Locale(""), 0.0
	for _, item := range strings.Split(header, ",") {
		fields := strings.Split(item, ";")
		q := 1.0
		for _, f := range fields[1:] {
			if v, ok := strings.CutPrefix(strings.TrimSpace(f), "q="); ok {
				parsed, e := strconv.ParseFloat(v, 64)
				if e != nil {
					parsed = 0
				}
				q = parsed
			}
		}
		l, ok := Parse(fields[0])
		if ok && q > bestQ {
			best, bestQ = l, q
		}
	}
	return best, best != ""
}

// First returns the first supported candidate, falling back to English.
func First(candidates ...Locale) Locale {
	for _, c := range candidates {
		switch c {
		case EN, ZHCN, ZHTW, JA:
			return c
		}
	}
	return EN
}
