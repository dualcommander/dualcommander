package commander

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"log/slog"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/lang"
	"fyne.io/fyne/v2/test"
)

func TestEnglishCatalogAndFallback(t *testing.T) {
	data, err := translations.ReadFile("translations/en.json")
	if err != nil {
		t.Fatal(err)
	}
	var messages map[string]json.RawMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if filepath.Ext(file) != ".go" || len(file) >= 8 && file[len(file)-8:] == "_test.go" {
			continue
		}
		node, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(node, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != "lang" || selector.Sel.Name != "L" {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if ok && literal.Kind == token.STRING {
				key, _ := strconv.Unquote(literal.Value)
				if _, exists := messages[key]; !exists {
					t.Errorf("%s: English catalog missing %q", file, key)
				}
			}
			return true
		})
	}
	if got := lang.L("Untranslated {{.Name}}", map[string]any{"Name": "example"}); got != "Untranslated example" {
		t.Fatalf("English template fallback: %q", got)
	}
}

func TestMenuUsesTranslations(t *testing.T) {
	if err := lang.AddTranslationsForLocale([]byte(`{"New Folder":"Translated folder","compact":"Translated compact"}`), fyne.Locale("en")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lang.AddTranslationsForLocale([]byte(`{"New Folder":"New Folder","compact":"compact"}`), fyne.Locale("en")); err != nil {
			t.Error(err)
		}
	})
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	u.settings()
	w := windowByTitle(t, u.app, "Settings")
	changed := false
	for _, object := range test.LaidOutObjects(w.Content()) {
		if choice, ok := object.(*dialogSelect); ok && slices.Contains(choice.Options, "Translated compact") {
			choice.SetSelected("Translated compact")
			changed = true
		}
	}
	if !changed || u.app.Preferences().String(prefDensity) != "compact" {
		t.Fatal("translated choice missing or changed persisted density identifier")
	}
	w.Close()
	for _, menu := range u.win.MainMenu().Items {
		for _, item := range menu.Items {
			if item.Label == "Translated folder" {
				return
			}
		}
	}
	t.Fatal("menu ignored translation bundle")
}

func TestUIErrorsUseStructuredLogging(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	root := fixture(t)
	u := newTestUI(t, test.NewTempApp(t), filepath.Join(root, "src"), filepath.Join(root, "dst"))
	u.showErr(nil)
	if output.Len() != 0 {
		t.Fatalf("nil error logged: %s", output.String())
	}
	u.showErr(errors.New("operation failed"))
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("missing structured error record: %q: %v", output.String(), err)
	}
	if record["level"] != "ERROR" || record["error"] != "operation failed" {
		t.Fatalf("wrong log fields: %v", record)
	}
}
