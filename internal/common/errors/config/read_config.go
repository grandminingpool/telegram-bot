package configErrors

import "fmt"

type ReadConfigError struct {
	ConfigName string
	Err        error
}

func (e *ReadConfigError) Error() string {
	return fmt.Sprintf("failed to read %s config: %s", e.ConfigName, e.Err.Error())
}
