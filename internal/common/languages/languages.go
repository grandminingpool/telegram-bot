package languages

import (
	"fmt"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/text/language"
)

type Languages struct {
	bundle            *i18n.Bundle
	fallbackLocale    language.Tag
	fallbackLocalizer *i18n.Localizer
	localizers        map[language.Tag]*i18n.Localizer
}

type LocalizersItem struct {
	Tag       language.Tag
	Localizer *i18n.Localizer
}

func (l *Languages) GetLocalizers() []LocalizersItem {
	localizers := make([]LocalizersItem, 0, len(l.localizers))

	for tag, localizer := range l.localizers {
		localizers = append(localizers, LocalizersItem{
			Tag:       tag,
			Localizer: localizer,
		})
	}

	return localizers
}

func (l *Languages) GetLocalizer(locale string) *i18n.Localizer {
	tag, err := language.Parse(locale)
	if err != nil {
		tag = l.fallbackLocale
	}

	localizer, ok := l.localizers[tag]
	if !ok {
		return l.fallbackLocalizer
	}

	return localizer
}

func LoadLanguages(localesPath string, locales []language.Tag) (*Languages, error) {
	fallbackLocale := language.English
	bundle := i18n.NewBundle(fallbackLocale)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	languages := &Languages{
		bundle:            bundle,
		fallbackLocale:    fallbackLocale,
		fallbackLocalizer: i18n.NewLocalizer(bundle, fallbackLocale.String()),
		localizers:        make(map[language.Tag]*i18n.Localizer),
	}

	for _, tag := range locales {
		file := fmt.Sprintf("active.%s.toml", tag.String())
		_, err := bundle.LoadMessageFile(fmt.Sprintf("%s/%s", localesPath, file))
		if err != nil {
			return nil, fmt.Errorf("failed to load message file (locales path: %s, file: %s), error: %w", localesPath, file, err)
		}

		localizer := i18n.NewLocalizer(bundle, tag.String())
		languages.localizers[tag] = localizer
	}

	return languages, nil
}
