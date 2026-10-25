package commander

import (
	"embed"
	"encoding/json"

	"fyne.io/fyne/v2/lang"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed translations/*.json
var translations embed.FS

var languageCodes = []string{"", "en", "zh-Hans", "es", "hi", "fr", "pt", "ru", "de", "ja", "ko", "it", "nl", "pl", "uk", "cs", "sk", "ro", "hu", "tr", "id", "vi", "th", "bn", "ta", "te", "mr", "sw", "fi", "sv", "da"}
var languageNames = []string{"System", "English", "简体中文", "Español", "हिन्दी", "Français", "Português", "Русский", "Deutsch", "日本語", "한국어", "Italiano", "Nederlands", "Polski", "Українська", "Čeština", "Slovenčina", "Română", "Magyar", "Türkçe", "Bahasa Indonesia", "Tiếng Việt", "ไทย", "বাংলা", "தமிழ்", "తెలుగు", "मराठी", "Kiswahili", "Suomi", "Svenska", "Dansk"}

// ponytail: language changes take effect on restart; no rebuilding open dialogs or jobs.
// Fyne has no public locale override; its built-in controls still follow the OS locale.
func languageTranslator(code string) func(string, ...any) string {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	files, _ := translations.ReadDir("translations")
	for _, file := range files {
		data, _ := translations.ReadFile("translations/" + file.Name())
		if _, err := bundle.ParseMessageFileBytes(data, file.Name()); err != nil {
			panic(err) // Embedded catalogs are validated by tests.
		}
	}
	localizer := i18n.NewLocalizer(bundle, code, "en")
	return func(key string, data ...any) string {
		config := &i18n.LocalizeConfig{DefaultMessage: &i18n.Message{ID: key, Other: key}}
		if len(data) > 0 {
			config.TemplateData = data[0]
		}
		text, _ := localizer.Localize(config)
		return text
	}
}

func init() {
	// Catalogs are embedded application resources; invalid catalogs are build defects.
	if err := lang.AddTranslationsFS(translations, "translations"); err != nil {
		panic(err)
	}
}
