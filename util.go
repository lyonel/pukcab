package main

import (
	"encoding/base64"
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
	regex = regexp.QuoteMeta(filter)
	regex = strings.Replace(regex, "\\?", ".", -1)
	regex = strings.Replace(regex, "\\*", ".*", -1)
	return
}
