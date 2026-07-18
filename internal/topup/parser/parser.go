package parser

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"outless/shared/vless"

	"gopkg.in/yaml.v3"
)

// Parse converts fetched content into validated vless:// URLs according to the configured parser type.
func Parse(ctx context.Context, content, parserType string, params map[string]any) ([]string, error) {
	switch parserType {
	case "vless_lines":
		return parseVlessLines(ctx, content)
	case "base64":
		return parseBase64(ctx, content)
	case "clash_yaml":
		return parseClashYAML(ctx, content)
	default:
		return nil, fmt.Errorf("unknown parser type %q", parserType)
	}
}

func parseVlessLines(ctx context.Context, content string) ([]string, error) {
	var urls []string
	sc := bufio.NewScanner(strings.NewReader(content))
	sc.Buffer(make([]byte, 1024), 1024*1024)

	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" || !strings.HasPrefix(line, "vless://") {
			continue
		}
		if _, err := vless.ParseURL(line); err != nil {
			continue
		}
		urls = append(urls, line)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading vless lines: %w", err)
	}
	return urls, nil
}

func parseBase64(ctx context.Context, content string) ([]string, error) {
	content = strings.TrimSpace(content)
	decoded, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return nil, fmt.Errorf("decoding base64 content: %w", err)
	}
	return parseVlessLines(ctx, string(decoded))
}

func parseClashYAML(ctx context.Context, content string) ([]string, error) {
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}

	dec := yaml.NewDecoder(strings.NewReader(content))
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decoding clash yaml: %w", err)
	}

	var urls []string
	for _, p := range doc.Proxies {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if p == nil {
			continue
		}
		u, err := vless.FromClash(p)
		if err != nil {
			continue
		}
		if _, err := vless.ParseURL(u); err != nil {
			continue
		}
		urls = append(urls, u)
	}

	if err := dec.Decode(&struct{}{}); err != nil && err != io.EOF {
		return nil, fmt.Errorf("trailing yaml content: %w", err)
	}
	return urls, nil
}
