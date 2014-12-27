package main

import (
	"encoding/base64"
	"strings"
)

func EncodeHash(h []byte) (hash string) {
	hash = base64.StdEncoding.EncodeToString(h)
	hash = strings.Replace(hash, "/", "_", -1)
	hash = strings.Replace(hash, "+", "-", -1)
	hash = strings.Trim(hash, "=")

	return hash
}
