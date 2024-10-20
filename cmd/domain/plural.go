// copy from github.com/dustin/go-humanize
package main

import (
	"strings"
)

// These are included because they are common technical terms.
var specialPlurals = map[string]string{
	"index":  "indices",
	"matrix": "matrices",
	"vertex": "vertices",
}

var sibilantEndings = []string{"s", "sh", "tch", "x"}

var isVowel = map[byte]bool{
	'A': true, 'E': true, 'I': true, 'O': true, 'U': true,
	'a': true, 'e': true, 'i': true, 'o': true, 'u': true,
}

func Plural(singular string) string {

	if plural := specialPlurals[singular]; plural != "" {
		return plural
	}
	for _, ending := range sibilantEndings {
		if strings.HasSuffix(singular, ending) {
			return singular + "es"
		}
	}
	l := len(singular)
	if l >= 2 && singular[l-1] == 'o' && !isVowel[singular[l-2]] {
		return singular + "es"
	}
	if l >= 2 && singular[l-1] == 'y' && !isVowel[singular[l-2]] {
		return singular[:l-1] + "ies"
	}

	return singular + "s"
}
