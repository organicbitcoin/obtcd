package main

import (
	"bytes"
	"fmt"
	"strings"
)

const semanticAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz-"

const (
	appMajor uint = 0
	appMinor uint = 1
	appPatch uint = 0

	appPreRelease = "alpha"
)

var appBuild string

func version() string {
	version := fmt.Sprintf("%d.%d.%d", appMajor, appMinor, appPatch)

	preRelease := normalizeVerString(appPreRelease)
	if preRelease != "" {
		version = fmt.Sprintf("%s-%s", version, preRelease)
	}

	build := normalizeVerString(appBuild)
	if build != "" {
		version = fmt.Sprintf("%s+%s", version, build)
	}

	return version
}

func normalizeVerString(str string) string {
	var result bytes.Buffer
	for _, r := range str {
		if strings.ContainsRune(semanticAlphabet, r) {
			_, _ = result.WriteRune(r)
		}
	}
	return result.String()
}
