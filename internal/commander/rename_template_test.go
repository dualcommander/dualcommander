package commander

import (
	"strings"
	"testing"
)

func TestRenameTemplate(t *testing.T) {
	for _, tc := range []struct{ pattern, output, name, want string }{
		{"{title}.{ext}", "{{.title}}.png", "cat.jpg", "cat.png"},
		{"{}.jpg", "{{.C0}}.png", "cat.jpg", "cat.png"},
		{"{}-{}.jpg", "{{.C1}}-{{.C0}}.png", "one-two.jpg", "two-one.png"},
		{"{}", "prefix_{{.C0}}_suffix", "a$b", "prefix_a$b_suffix"},
		{"{}", "${{.C0}}", "x", "$x"},
		{"{}", "{{.C0}}", ".9router", ".9router"},
		{"{}.jpg", "{{.C0}}.png", "cat.jpg.bak", "cat.jpg.bak"},
		{"{}", "{{.C0}}", "猫\n犬", "猫\n犬"},
		{"{name}.{ext}", "{{upper .name}}.{{lower .ext}}", "été.JPG", "ÉTÉ.jpg"},
		{"{name}.{ext}", "{{kebab .name}}.{{lower .ext}}", "CaaZaa.JPG", "caa-zaa.jpg"},
		{"{name}.{ext}", "{{snake .name}}.{{.ext}}", "HTTPServer.jpg", "http_server.jpg"},
		{"{name}", "{{camel .name}}", "caa_zaa", "caaZaa"},
		{"{name}", "{{pascal .name}}", "caa-zaa", "CaaZaa"},
		{"{name}", "{{snake .name}}", "Version2ID", "version2_id"},
		{"{name}", "{{kebab .name}}", "ÉtéPhoto", "été-photo"},
		{"{name}", "{{snake .name}}", "hello world-test", "hello_world_test"},
		{"{name}", "{{lower .name}}", ".ÉTÉ", ".été"},
		{"{name}.{ext}", "{{upper .name}}.{{lower .ext}}", ".JPG", ".jpg"},
	} {
		t.Run(tc.name+tc.output, func(t *testing.T) {
			r, replacement, err := renameTemplate(tc.pattern, tc.output)
			if err != nil {
				t.Fatal(err)
			}
			got := tc.name
			if groups := r.FindStringSubmatch(tc.name); groups != nil {
				values := map[string]string{}
				for i, key := range r.SubexpNames()[1:] {
					values[key] = groups[i+1]
				}
				var b strings.Builder
				if err := replacement.Execute(&b, values); err != nil {
					t.Fatal(err)
				}
				got = b.String()
			}
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
	for _, tc := range [][2]string{{"", "{{.C0}}"}, {"{}", "{{.C1}}"}, {"{}", "{{.missing}}"}, {"{", ""}, {"{}", "{{"}, {"{}", "{{if .C0}}x{{end}}"}, {"{}", "{{range .C0}}x{{end}}"}, {"{}", "{{printf .C0}}"}, {"{}", "{{.C0.Bad}}"}, {"{x}-{x}", ""}} {
		if _, _, err := renameTemplate(tc[0], tc[1]); err == nil {
			t.Fatalf("accepted %v", tc)
		}
	}
}

func TestRenameTemplateRejectsUnsafeHelperCalls(t *testing.T) {
	for _, output := range []string{
		"{{upper}}", "{{upper .name .ext}}", "{{upper .missing}}", "{{upper .name.Bad}}",
		"{{upper \"literal\"}}", "{{printf .name}}", "{{html .name}}", "{{.name | upper}}",
		"{{upper (lower .name)}}", "{{$x := .name}}", "{{if .name}}x{{end}}",
	} {
		if _, _, err := renameTemplate("{name}.{ext}", output); err == nil {
			t.Errorf("accepted unsupported expression %s", output)
		}
	}
}

func TestRenameTemplateDiagnosticContext(t *testing.T) {
	_, _, unknown := renameTemplate("{name}", "{{.missing}}")
	_, _, duplicate := renameTemplate("{name}-{name}", "")
	if unknown == nil || !strings.Contains(unknown.Error(), "missing") {
		t.Fatal("unknown capture diagnostic must identify missing capture")
	}
	if duplicate == nil || !strings.Contains(duplicate.Error(), "name") {
		t.Fatal("duplicate capture diagnostic must identify duplicate")
	}
}

func TestRenameTemplateIndex(t *testing.T) {
	for _, output := range []string{"{{.Index}}", `{{printf "%02d" .Index}}`, `{{printf "%03d" .Index}}`} {
		_, tpl, err := renameTemplate("{name}", output)
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		if err := tpl.Execute(&b, map[string]any{"name": "photo", "Index": 12}); err != nil {
			t.Fatal(err)
		}
		want := "12"
		if strings.Contains(output, "03") {
			want = "012"
		}
		if b.String() != want {
			t.Fatalf("%s: got %q want %q", output, b.String(), want)
		}
	}
	for _, output := range []string{`{{printf "%s" .Index}}`, `{{printf "%099999d" .Index}}`, `{{printf "%02d" .name}}`, `{{upper .Index}}`, `{{printf "%02d" .Index .Index}}`} {
		if _, _, err := renameTemplate("{name}", output); err == nil {
			t.Errorf("accepted %s", output)
		}
	}
	if _, _, err := renameTemplate("{Index}", "{{.Index}}"); err == nil {
		t.Fatal("reserved capture accepted")
	}
}

func TestRenameTemplateIndexFunction(t *testing.T) {
	for _, tc := range []struct{ output, want string }{
		{`{{index}}`, "12"},
		{`{{index 1}}`, "12"},
		{`{{index 3}}-{{index 2}}`, "012-12"},
		{`{{.name}}_{{index 9}}`, "photo_000000012"},
	} {
		_, tpl, err := renameTemplate("{name}", tc.output)
		if err != nil {
			t.Errorf("%s: %v", tc.output, err)
			continue
		}
		var b strings.Builder
		if err := tpl.Execute(&b, map[string]any{"name": "photo", "Index": 12}); err != nil {
			t.Fatal(err)
		}
		if b.String() != tc.want {
			t.Errorf("%s: got %q want %q", tc.output, b.String(), tc.want)
		}
	}
	for _, output := range []string{`{{index 0}}`, `{{index 10}}`, `{{index -1}}`, `{{index 2.0}}`, `{{index "3"}}`, `{{index .name}}`, `{{index 2 3}}`, `{{index .Index}}`, `{{index | upper}}`} {
		if _, _, err := renameTemplate("{name}", output); err == nil {
			t.Errorf("accepted %s", output)
		}
	}
}
