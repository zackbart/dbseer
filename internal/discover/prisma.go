package discover

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var (
	datasourceRe = regexp.MustCompile(`(?s)datasource\s+\w+\s*\{([^}]+)\}`)
	urlRe        = regexp.MustCompile(`url\s*=\s*(?:env\("([^"]+)"\)|"([^"]+)")`)
	providerRe   = regexp.MustCompile(`provider\s*=\s*"([^"]+)"`)
)

// ParsePrismaDatasource parses a Prisma schema file and extracts the datasource
// provider hint, connection URL or env var reference.
//
// If the URL is specified as env("VAR"), envVar is populated with the variable name
// and url is empty — the caller should resolve via LookupEnv. If the URL is a literal,
// url is populated directly and envVar is empty.
//
// Returns an error if the file cannot be read or contains no datasource block.
func ParsePrismaDatasource(schemaPath string) (url, providerHint string, envVar string, err error) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return "", "", "", fmt.Errorf("prisma: reading %s: %w", schemaPath, err)
	}

	// Strip single-line // comments before regex matching.
	cleaned := stripLineComments(string(data))

	block := datasourceRe.FindStringSubmatch(cleaned)
	if block == nil {
		return "", "", "", fmt.Errorf("prisma: no datasource block in %s", schemaPath)
	}
	blockContent := block[1]

	// Extract provider.
	pm := providerRe.FindStringSubmatch(blockContent)
	if pm != nil {
		providerHint = pm[1]
	}

	// Extract url.
	um := urlRe.FindStringSubmatch(blockContent)
	if um == nil {
		return "", providerHint, "", fmt.Errorf("prisma: no url field in datasource block in %s", schemaPath)
	}

	if um[1] != "" {
		// env("VAR") reference.
		return "", providerHint, um[1], nil
	}
	// Literal URL.
	return um[2], providerHint, "", nil
}

// stripLineComments removes // single-line comments from a string, preserving
// line structure so that regex line-anchors still work if needed.
// It only strips // sequences that appear before any double-quoted string on the
// same line, to avoid truncating URLs like postgres://... inside string values.
func stripLineComments(src string) string {
	var b strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(src))
	for scanner.Scan() {
		line := scanner.Text()
		line = removeLineComment(line)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// removeLineComment strips a trailing // comment from a single line of Prisma
// schema text. It is quote-aware: // inside a double-quoted string is preserved.
func removeLineComment(line string) string {
	inStr := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == '"' {
			inStr = !inStr
			continue
		}
		if !inStr && ch == '/' && i+1 < len(line) && line[i+1] == '/' {
			return line[:i]
		}
	}
	return line
}
