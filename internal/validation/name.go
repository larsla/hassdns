package validation

import "regexp"

func ValidateName(name string) bool {
	v, _ := regexp.MatchString("^[a-z0-9]+$", name)
	return v
}
