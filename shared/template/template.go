package template

import (
	"fmt"
	"outless/internal/country"
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
	Group        string
	User         string
	Lifetime     string
	DeletionDate string
}

// TemplateData holds all available variables for template rendering.
type TemplateData struct {
	VLESS VLESSData
}

// BuildTemplateData creates TemplateData from VLESS data, Node, and Token.
func BuildTemplateData(
	vless VLESSData,
	nodeCountry,
	nodeCountryShort,
	nodeGroupID,
	tokenOwner string,
	tokenCreatedAt,
	tokenExpiresAt time.Time,
) TemplateData {
	vless.Country = nodeCountry
	vless.CountryShort = strings.ToUpper(nodeCountryShort)
	vless.CountryFlag = country.FlagEmoji(nodeCountryShort)
	vless.Group = nodeGroupID
	vless.User = tokenOwner
	vless.DeletionDate = formatDeletionDate(tokenExpiresAt)
	vless.Lifetime = formatLifetime(tokenCreatedAt, tokenExpiresAt)
	return TemplateData{VLESS: vless}
}

// RenderTemplate replaces {{variable}} placeholders with values from TemplateData.
func RenderTemplate(tmpl string, data TemplateData) string {
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
		if fallback != "" {
			if strings.HasPrefix(fallback, `"`) && strings.HasSuffix(fallback, `"`) {
				return strings.Trim(fallback, `"`)
			}
			if val := getFieldValue(fallback, data); val != "" {
				return val
			}
		}
		return match
	})
}

func formatDeletionDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}

func formatLifetime(createdAt, expiresAt time.Time) string {
	if createdAt.IsZero() || expiresAt.IsZero() {
		return ""
	}
	total := expiresAt.Sub(createdAt)
	if total <= 0 {
		return "0d 0h"
	}
	totalHours := int(total.Hours())
	return fmt.Sprintf("%dd %dh", totalHours/24, totalHours%24)
}

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
		case "country_flag", "flag":
			return data.VLESS.CountryFlag
		case "group":
			return data.VLESS.Group
		case "user":
			return data.VLESS.User
		case "lifetime":
			return data.VLESS.Lifetime
		case "deletion_date":
			return data.VLESS.DeletionDate
		}
	}
	return ""
}
