package main

import (
	"encoding/base64"
	"path/filepath"
	"regexp"
	"strings"
)

func EncodeHash(h []byte) (hash string) {
	hash = base64.StdEncoding.EncodeToString(h)
	hash = strings.Replace(hash, "/", "_", -1)
	hash = strings.Replace(hash, "+", "-", -1)
	hash = strings.Trim(hash, "=")

	return hash
}

func ConvertGlob(filter string) (regex string) {
	regex = regexp.QuoteMeta(strings.TrimRight(filter, string(filepath.Separator)))
	regex = strings.Replace(regex, "\\?", ".", -1)
	regex = strings.Replace(regex, "\\*", ".*", -1)

	if len(regex) > 0 && regex[0] == filepath.Separator {
		regex = "^" + regex
	} else {
		regex = string(filepath.Separator) + regex
	}
	regex = regex + "(" + string(filepath.Separator) + ".*)?$"

	return
}
