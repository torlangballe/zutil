package zstr

import (
	"fmt"
	"strings"
)

func IndexOfRuneInSet(r rune, set []rune) int {
	for i, s := range set {
		if s == r {
			return i
		}
	}
	return -1
}

func BreakRunesIntoLines(runes []rune, breakChars string, columns int) (lines [][]rune) {
	if breakChars == "" {
		breakChars = "\n\r –-\t"
	}
	lastBreak := -1
	var count int
	for {
		added := false
		for i, r := range runes {
			if strings.IndexRune(breakChars, r) != -1 {
				lastBreak = i
			}
			if i >= columns {
				if lastBreak == -1 || i-lastBreak > columns/3 {
					lastBreak = i
				}
				line := runes[:lastBreak]
				runes = runes[lastBreak:]
				lines = append(lines, line)
				added = true
				break
			}
		}
		if !added {
			break
		}
		count++
		if count > 50 {
			fmt.Println("BreakRunesIntoLines loop!", string(runes), breakChars, columns)
			break
		}
	}
	if len(runes) > 0 {
		lines = append(lines, runes)
	}
	return
}
