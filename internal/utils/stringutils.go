package utils

import (
	"strings"
)

func IsStringEmpty(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}

func RemoveEmptyEntries(s []string) []string {

	var r []string
	for _, str := range s {
		if !IsStringEmpty(str) {
			r = append(r, str)
		}
	}

	return r
}
