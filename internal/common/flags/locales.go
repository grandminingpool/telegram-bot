package flags

import (
	"fmt"
	"strings"

	"golang.org/x/text/language"
)

type Locales []language.Tag

func (l Locales) String() string {
	strValues := make([]string, 0, len(l))
	for _, tag := range l {
		strValues = append(strValues, tag.String())
	}

	return strings.Join(strValues, ",")
}

func (l *Locales) Set(value string) error {
	locales := []language.Tag{}
	for _, localeVal := range strings.Split(value, ",") {
		tag, err := language.Parse(localeVal)
		if err != nil {
			return fmt.Errorf("invalid locale value: %s, error: %w", localeVal, err)
		}

		locales = append(locales, tag)
	}

	*l = locales

	return nil
}
