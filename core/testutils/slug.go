package testutils

import (
	"regexp"
	"strings"
)

// Slug takes a human-readable string and returns a filename-friendly slug.
func Slug(input string) string {
	// Convert to lowercase
	slug := strings.ToLower(input)

	// Replace spaces and underscores with dashes
	slug = strings.ReplaceAll(slug, " ", "_")
	slug = strings.ReplaceAll(slug, "-", "_")

	// Remove all non-alphanumeric and non-underscore characters
	re := regexp.MustCompile(`[^a-z0-9\_]+`)
	slug = re.ReplaceAllString(slug, "")

	return slug
}
