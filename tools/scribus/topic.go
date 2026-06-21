package scribus

import "strings"

// matchTopic reports whether an AMQP topic pattern matches a routing key.
// '*' matches exactly one word, '#' matches zero or more words.
func matchTopic(pattern, key string) bool {
	return matchSegs(strings.Split(pattern, "."), strings.Split(key, "."))
}

func matchSegs(p, k []string) bool {
	for len(p) > 0 {
		switch p[0] {
		case "#":
			if len(p) == 1 {
				return true
			}
			for i := 0; i <= len(k); i++ {
				if matchSegs(p[1:], k[i:]) {
					return true
				}
			}
			return false
		case "*":
			if len(k) == 0 {
				return false
			}
		default:
			if len(k) == 0 || p[0] != k[0] {
				return false
			}
		}
		p, k = p[1:], k[1:]
	}
	return len(k) == 0
}

// overlap reports whether some routing key matches both topic patterns.
func overlap(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	memo := make(map[[2]int]bool)
	var rec func(i, j int) bool
	rec = func(i, j int) bool {
		if i == len(pa) && j == len(pb) {
			return true
		}
		key := [2]int{i, j}
		if v, ok := memo[key]; ok {
			return v
		}
		res := false
		if i < len(pa) && pa[i] == "#" && rec(i+1, j) {
			res = true
		}
		if !res && j < len(pb) && pb[j] == "#" && rec(i, j+1) {
			res = true
		}
		if !res && i < len(pa) && j < len(pb) {
			wa, wb := pa[i], pb[j]
			compatible := wa == "*" || wa == "#" || wb == "*" || wb == "#" || wa == wb
			ni, nj := i+1, j+1
			if wa == "#" {
				ni = i
			}
			if wb == "#" {
				nj = j
			}
			if compatible && (ni > i || nj > j) {
				res = rec(ni, nj)
			}
		}
		memo[key] = res
		return res
	}
	return rec(0, 0)
}
