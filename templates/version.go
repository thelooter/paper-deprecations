package templates

import (
	"strconv"
	"strings"
)

// CompareVersions returns true if version a is newer than version b
func CompareVersions(a, b string) bool {
	aParts := strings.Split(strings.TrimPrefix(a, "v"), ".")
	bParts := strings.Split(strings.TrimPrefix(b, "v"), ".")

	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		aNum, _ := strconv.Atoi(aParts[i])
		bNum, _ := strconv.Atoi(bParts[i])
		if aNum != bNum {
			return aNum > bNum
		}
	}
	return len(aParts) > len(bParts)
}
