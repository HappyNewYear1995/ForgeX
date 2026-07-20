package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// versionRegex matches AA.BB.CC.DD where each segment is a non-negative integer
var versionRegex = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)\.(\d+)$`)

// ValidateVersion checks if the version string matches AA.BB.CC.DD format.
// Returns nil if valid, or an error describing the issue.
func ValidateVersion(v string) error {
	if v == "" {
		return nil // empty is allowed (will use default)
	}
	matches := versionRegex.FindStringSubmatch(v)
	if matches == nil {
		return fmt.Errorf("版本号格式错误，应为 AA.BB.CC.DD（如 3.0.1.2）")
	}
	// Validate each segment is within reasonable range (0-9999)
	for i, seg := range matches[1:] {
		n, _ := strconv.Atoi(seg)
		if n > 9999 {
			labels := []string{"大版本号(AA)", "ECR版本号(BB)", "TCN版本号(CC)", "小版本号(DD)"}
			return fmt.Errorf("%s 不能超过 9999", labels[i])
		}
	}
	return nil
}

// ValidateVersionRequired is like ValidateVersion but also rejects empty strings.
func ValidateVersionRequired(v string) error {
	if strings.TrimSpace(v) == "" {
		return fmt.Errorf("版本号不能为空")
	}
	return ValidateVersion(v)
}

// IncrementVersion increments the 4th segment (DD) of a version string AA.BB.CC.DD.
// If the version is empty or invalid, returns "0.0.0.1".
func IncrementVersion(version string) string {
	matches := versionRegex.FindStringSubmatch(version)
	if matches == nil {
		return "0.0.0.1"
	}
	aa, _ := strconv.Atoi(matches[1])
	bb, _ := strconv.Atoi(matches[2])
	cc, _ := strconv.Atoi(matches[3])
	dd, _ := strconv.Atoi(matches[4])
	dd++
	return fmt.Sprintf("%d.%d.%d.%d", aa, bb, cc, dd)
}
