package cache

import (
	"crypto/md5"
	"fmt"
	"sort"
	"strings"
)

// GenerateEphemerisKey creates cache key for ephemeris data
func GenerateEphemerisKey(operation string, params map[string]interface{}) string {
	// Sort parameters for consistent key generation
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, params[key]))
	}

	paramStr := strings.Join(parts, "&")
	hash := md5.Sum([]byte(paramStr))

	return fmt.Sprintf("eph:%s:%x", operation, hash)
}

// GenerateCacheKey creates a generic cache key with namespace
func GenerateCacheKey(namespace, operation string, params map[string]interface{}) string {
	// Sort parameters for consistent key generation
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, params[key]))
	}

	paramStr := strings.Join(parts, "&")
	hash := md5.Sum([]byte(paramStr))

	return fmt.Sprintf("%s:%s:%x", namespace, operation, hash)
}
