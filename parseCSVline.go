package mv370

import (
	"strings"
)

// ParseCSVLine parses a single CSV line and removes optional quotes per field without using the csv library.
func ParseCSVLine(line string) ([]string, error) {
	var fields []string
	if line == "" {
		return fields, nil
	}

	var field strings.Builder
	inQuotes := false

	for i, char := range line {
		switch char {
		case '"':
			if inQuotes && i+1 < len(line) && rune(line[i+1]) == '"' {
				// Handle escaped quote by adding a single quote to the field
				field.WriteRune('"')
				i++ // Skip the next quote
			} else {
				// Toggle inQuotes flag
				inQuotes = !inQuotes
			}
		case ',':
			if inQuotes {
				// Add the comma as part of the field
				field.WriteRune(char)
			} else {
				// End of field
				fields = append(fields, field.String())
				field.Reset()
			}
		default:
			// Add the character to the field
			field.WriteRune(char)
		}
	}

	// Add the last field
	fields = append(fields, field.String())

	// Trim optional quotes around each field
	for i := range fields {
		fields[i] = strings.Trim(fields[i], `"`)
	}

	return fields, nil
}
