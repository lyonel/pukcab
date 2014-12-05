package main

import (
	"fmt"
	"strconv"
	"strings"
)

// paxHeader formats a single pax record, prefixing it with the appropriate length
func paxHeader(msg string) string {
	const padding = 2 // Extra padding for space and newline
	size := len(msg) + padding
	size += len(strconv.Itoa(size))
	record := fmt.Sprintf("%d %s\n", size, msg)
	if len(record) != size {
		// Final adjustment if adding size increased
		// the number of digits in size
		size = len(record)
		record = fmt.Sprintf("%d %s\n", size, msg)
	}
	return record
}

func paxHeaders(headers map[string]interface{}) []byte {
	result := ""

	for k, v := range headers {
		if k[0] == '.' {
			k = strings.ToUpper(programName) + k
		}
		result = result + paxHeader(k+"="+fmt.Sprintf("%v", v))
	}

	return []byte(result)
}
