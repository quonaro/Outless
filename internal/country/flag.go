package country

import "strings"

// FlagEmoji converts a two-letter ISO country code to a flag emoji.
func FlagEmoji(code string) string {
	if len(code) != 2 {
		return "🏳️"
	}
	code = strings.ToUpper(code)
	first := rune(code[0])
	second := rune(code[1])
	if first < 'A' || first > 'Z' || second < 'A' || second > 'Z' {
		return "🏳️"
	}
	const regionalIndicatorA = rune(0x1F1E6)
	return string([]rune{regionalIndicatorA + (first - 'A'), regionalIndicatorA + (second - 'A')})
}
