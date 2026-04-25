package template

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// VLESSData holds parsed VLESS URL data and node metadata.
type VLESSData struct {
	Name         string
	Host         string
	Port         int
	SNI          string
	Security     string
	Encryption   string
	Flow         string
	FP           string
	Country      string
	CountryShort string
	CountryFlag  string
	Ping         string
	Group        string
	User         string
}

// TemplateData holds all available variables for template rendering.
type TemplateData struct {
	VLESS VLESSData
}

// BuildTemplateData creates TemplateData from VLESS data, Node, and Token.
func BuildTemplateData(vless VLESSData, nodeCountry, nodeCountryShort, nodeGroupID string, nodeLatency time.Duration, tokenOwner string) TemplateData {
	vless.Country = nodeCountry
	vless.CountryShort = strings.ToUpper(nodeCountryShort)
	vless.CountryFlag = countryFlagEmoji(nodeCountryShort)
	vless.Ping = strconv.FormatInt(nodeLatency.Milliseconds(), 10)
	vless.Group = nodeGroupID
	vless.User = tokenOwner

	return TemplateData{VLESS: vless}
}

// RenderTemplate replaces {{variable}} placeholders with values from TemplateData.
// Supports:
// - {{var}} - simple substitution
// - {{var|"default"}} - fallback to string literal
// - {{var|other_var}} - fallback to another variable
func RenderTemplate(tmpl string, data TemplateData) string {
	// Regex matches {{variable}} or {{variable|fallback}}
	re := regexp.MustCompile(`{{([a-zA-Z0-9_.]+)(?:\|([^{} ]+))?}}`)

	return re.ReplaceAllStringFunc(tmpl, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		key := submatches[1]
		fallback := ""
		if len(submatches) > 2 {
			fallback = submatches[2]
		}

		val := getFieldValue(key, data)
		if val != "" {
			return val
		}

		// Handle fallback
		if fallback != "" {
			// Check if fallback is a quoted string literal
			if strings.HasPrefix(fallback, `"`) && strings.HasSuffix(fallback, `"`) {
				return strings.Trim(fallback, `"`)
			}
			// Try as another variable
			if val := getFieldValue(fallback, data); val != "" {
				return val
			}
		}

		// Return original if no value found
		return match
	})
}

// getFieldValue retrieves a field value from TemplateData by dot notation.
func getFieldValue(key string, data TemplateData) string {
	parts := strings.Split(key, ".")
	if len(parts) == 0 {
		return ""
	}

	switch parts[0] {
	case "vless":
		if len(parts) < 2 {
			return ""
		}
		switch parts[1] {
		case "name":
			return data.VLESS.Name
		case "host", "ip":
			return data.VLESS.Host
		case "port":
			return strconv.Itoa(data.VLESS.Port)
		case "sni":
			return data.VLESS.SNI
		case "security":
			return data.VLESS.Security
		case "encryption":
			return data.VLESS.Encryption
		case "flow":
			return data.VLESS.Flow
		case "fp":
			return data.VLESS.FP
		case "country":
			return data.VLESS.Country
		case "country_short":
			return data.VLESS.CountryShort
		case "country_flag":
			return data.VLESS.CountryFlag
		case "ping":
			return data.VLESS.Ping
		case "group":
			return data.VLESS.Group
		case "user":
			return data.VLESS.User
		}
	}

	return ""
}

// countryFlagEmoji converts a 2-letter country code to flag emoji.
func countryFlagEmoji(code string) string {
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
	return string([]rune{
		regionalIndicatorA + (first - 'A'),
		regionalIndicatorA + (second - 'A'),
	})
}
