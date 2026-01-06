package main

import "strings"

func matchesSearch(text string, search string) (match bool) {
	if search == "" || strings.Contains(text, search) {
		return true
	}

	return false
}
