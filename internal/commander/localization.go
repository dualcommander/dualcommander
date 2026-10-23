package commander

import (
	"embed"

	"fyne.io/fyne/v2/lang"
)

//go:embed translations/*.json
var translations embed.FS

func init() {
	// Catalogs are embedded application resources; invalid catalogs are build defects.
	if err := lang.AddTranslationsFS(translations, "translations"); err != nil {
		panic(err)
	}
}
