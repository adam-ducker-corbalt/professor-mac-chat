// Package envconfig reads configuration from environment variables. It
// mirrors ProfessorMAC's internal/envconfig, so code using it moves over as-is.
package envconfig

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func String(name string) string {
	return os.Getenv(name)
}

func StringDefault(name string, def string) string {
	if os.Getenv(name) != "" {
		return os.Getenv(name)
	}
	return def
}

func BoolDefault(name string, def bool) (bool, error) {
	if os.Getenv(name) == "" {
		return def, nil
	}
	val, err := strconv.ParseBool(os.Getenv(name))
	if err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}
	return val, nil
}

// Required returns an error naming every variable that is unset or empty.
func Required(names ...string) error {
	var missing []string
	for _, name := range names {
		if os.Getenv(name) == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}
	return nil
}
