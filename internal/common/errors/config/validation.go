package configErrors

import "fmt"

type ValidationError struct {
	ConfigName string
	Err        error
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("failed to validate %s config: %s", e.ConfigName, e.Err.Error())
}
