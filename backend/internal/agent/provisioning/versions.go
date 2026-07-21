package provisioning

import (
	"bufio"
	_ "embed"
	"regexp"
	"strings"
)

//go:embed agent-cli-versions.env
var cliVersionManifest string

var semanticVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$`)

func MustCLIVersion(key string) string {
	scanner := bufio.NewScanner(strings.NewReader(cliVersionManifest))
	for scanner.Scan() {
		name, value, ok := strings.Cut(strings.TrimSpace(scanner.Text()), "=")
		if ok && name == key {
			value = strings.TrimSpace(value)
			if semanticVersionPattern.MatchString(value) {
				return value
			}
			panic("invalid " + key + " in agent-cli-versions.env")
		}
	}
	panic("missing " + key + " in agent-cli-versions.env")
}
