package formatUtils

import (
	"strings"
	"time"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

func BoolText(value bool, l *i18n.Localizer) string {
	msgID := "No"

	if value {
		msgID = "Yes"
	}

	return l.MustLocalize(&i18n.LocalizeConfig{
		MessageID: msgID,
	})
}

func UptimeText(t time.Time, l *i18n.Localizer) string {
	uptimeTextItems := []string{}
	elapsed := time.Since(t)
	hours := elapsed.Hours()
	days := hours / 24
	minutes := elapsed.Minutes()

	if days > 0 {
		uptimeTextItems = append(uptimeTextItems, l.MustLocalize(&i18n.LocalizeConfig{
			MessageID:   "Day",
			PluralCount: days,
		}))
	}

	if hours > 0 {
		uptimeTextItems = append(uptimeTextItems, l.MustLocalize(&i18n.LocalizeConfig{
			MessageID:   "Hour",
			PluralCount: hours,
		}))
	}

	if minutes > 0 {
		uptimeTextItems = append(uptimeTextItems, l.MustLocalize(&i18n.LocalizeConfig{
			MessageID:   "Minute",
			PluralCount: minutes,
		}))
	}

	return strings.Join(uptimeTextItems, ", ")
}
