package configErrors

import "fmt"

type UnmarshalError struct {
	ConfigName string
	Err        error
}

func (e *UnmarshalError) Error() string {
	return fmt.Sprintf("unable to decode into struct in %s config, error: %s", e.ConfigName, e.Err.Error())
}
