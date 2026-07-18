package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
)

// FetchURLContent fetches content from a URL and returns it as a string.
func FetchURLContent(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "ContentNode/1.0")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// RewritePlaylist rewrites HLS playlist segment lines to use CDN domains.
// Segment lines are rewritten to: https://{domain}/{slug}/{filename}
// Multiple domains are load-balanced via round-robin.
func RewritePlaylist(content string, domains []string, slug string) string {
	if len(domains) == 0 {
		return content
	}

	var sb strings.Builder
	lines := strings.Split(content, "\n")
	domainIndex := 0
	maxDomainIndex := len(domains) - 1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "#") {
			sb.WriteString(line)
			sb.WriteString("\n")
		} else {
			// Segment line (e.g., v-1234.ts or fragment.ts)
			filename := path.Base(trimmed)

			sb.WriteString("https://")
			sb.WriteString(domains[domainIndex])
			sb.WriteString("/")
			sb.WriteString(slug)
			sb.WriteString("/")
			sb.WriteString(filename)
			sb.WriteString("\n")

			// Round-robin domain selection
			if maxDomainIndex > 0 {
				domainIndex = (domainIndex + 1) % len(domains)
			}
		}
	}

	return sb.String()
}
