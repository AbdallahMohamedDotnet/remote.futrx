package containers

import (
	"regexp"
	"strconv"
)

var semanticVersionPattern = regexp.MustCompile(`\b(\d+)\.(\d+)\.(\d+)(-([0-9A-Za-z.-]+))?`)

type semanticVersion struct {
	major      int
	minor      int
	patch      int
	prerelease string
}

func parseSemanticVersion(value string) (semanticVersion, bool) {
	match := semanticVersionPattern.FindStringSubmatch(value)
	if match == nil {
		return semanticVersion{}, false
	}
	major, errMajor := strconv.Atoi(match[1])
	minor, errMinor := strconv.Atoi(match[2])
	patch, errPatch := strconv.Atoi(match[3])
	if errMajor != nil || errMinor != nil || errPatch != nil {
		return semanticVersion{}, false
	}
	return semanticVersion{major: major, minor: minor, patch: patch, prerelease: match[5]}, true
}

func semanticVersionAtLeast(actualOutput, minimumValue string) bool {
	actual, ok := parseSemanticVersion(actualOutput)
	if !ok {
		return false
	}
	minimum, ok := parseSemanticVersion(minimumValue)
	if !ok {
		return false
	}
	actualCore := [3]int{actual.major, actual.minor, actual.patch}
	minimumCore := [3]int{minimum.major, minimum.minor, minimum.patch}
	for i := range actualCore {
		if actualCore[i] != minimumCore[i] {
			return actualCore[i] > minimumCore[i]
		}
	}
	if actual.prerelease == minimum.prerelease {
		return true
	}
	// A stable release is newer than a prerelease with the same core.
	if actual.prerelease == "" {
		return true
	}
	if minimum.prerelease == "" {
		return false
	}
	return actual.prerelease >= minimum.prerelease
}
