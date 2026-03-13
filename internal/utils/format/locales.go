package format

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
	totalMinutes := int(elapsed.Minutes())
	days := totalMinutes / (60 * 24)
	hours := (totalMinutes % (60 * 24)) / 60
	minutes := totalMinutes % 60

	if days > 0 {
		uptimeTextItems = append(uptimeTextItems, l.MustLocalize(&i18n.LocalizeConfig{
			MessageID:   "Day",
			PluralCount: days,
			TemplateData: map[string]interface{}{
				"Count": days,
			},
		}))
	}

	if hours > 0 {
		uptimeTextItems = append(uptimeTextItems, l.MustLocalize(&i18n.LocalizeConfig{
			MessageID:   "Hour",
			PluralCount: hours,
			TemplateData: map[string]interface{}{
				"Count": hours,
			},
		}))
	}

	if minutes > 0 {
		uptimeTextItems = append(uptimeTextItems, l.MustLocalize(&i18n.LocalizeConfig{
			MessageID:   "Minute",
			PluralCount: minutes,
			TemplateData: map[string]interface{}{
				"Count": minutes,
			},
		}))
	}

	return strings.Join(uptimeTextItems, ", ")
}
