package commander

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"text/template"
	"text/template/parse"

	"fyne.io/fyne/v2/lang"
	"github.com/huandu/xstrings"
)

// renameTemplate matches entire names; output permits captures, case helpers, and numbering.
func renameTemplate(pattern, output string) (*regexp.Regexp, *template.Template, error) {
	invalid := func(reason string) (*regexp.Regexp, *template.Template, error) {
		return nil, nil, errors.New(reason)
	}
	if pattern == "" {
		return invalid(lang.L("Enter a match template."))
	}
	var expression strings.Builder
	expression.WriteString("^(?s:")
	names := map[string]string{}
	for index := 0; len(pattern) > 0; {
		start := strings.IndexByte(pattern, '{')
		if start < 0 {
			if strings.Contains(pattern, "}") {
				return invalid(lang.L("Unbalanced braces in match template."))
			}
			expression.WriteString(regexp.QuoteMeta(pattern))
			break
		}
		if strings.Contains(pattern[:start], "}") {
			return invalid(lang.L("Unbalanced braces in match template."))
		}
		expression.WriteString(regexp.QuoteMeta(pattern[:start]))
		end := strings.IndexByte(pattern[start:], '}')
		if end < 0 {
			return invalid(lang.L("Unbalanced braces in match template."))
		}
		key := pattern[start+1 : start+end]
		if key == "" {
			key = fmt.Sprintf("C%d", index)
		}
		if key == "Index" || !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`).MatchString(key) {
			return invalid(fmt.Sprintf(lang.L("Invalid capture name: %s"), key))
		}
		if _, exists := names[key]; exists {
			return invalid(fmt.Sprintf(lang.L("Duplicate capture: %s"), key))
		}
		names[key] = ""
		expression.WriteString("(?P<" + key + ">.*?)")
		index++
		pattern = pattern[start+end+1:]
	}
	expression.WriteString(")$")
	r, err := regexp.Compile(expression.String())
	if err != nil {
		return nil, nil, err
	}
	helpers := template.FuncMap{
		"index": func(value int, width ...int) string {
			digits := 0
			if len(width) > 0 {
				digits = width[0]
			}
			return fmt.Sprintf("%0*d", digits, value)
		},
		"upper": strings.ToUpper, "lower": strings.ToLower,
		"snake": xstrings.ToSnakeCase, "kebab": xstrings.ToKebabCase,
		"camel": xstrings.ToCamelCase, "pascal": xstrings.ToPascalCase,
	}
	t, err := template.New("name").Funcs(helpers).Option("missingkey=error").Parse(output)
	if err != nil {
		return nil, nil, err
	}
	if len(t.Templates()) != 1 {
		return invalid(lang.L("Use text, captures, or upper/lower/snake/kebab/camel/pascal with one capture."))
	}
	for _, node := range t.Tree.Root.Nodes {
		switch n := node.(type) {
		case *parse.TextNode:
		case *parse.ActionNode:
			if len(n.Pipe.Decl) != 0 || len(n.Pipe.Cmds) != 1 {
				return invalid(lang.L("Use text, captures, or upper/lower/snake/kebab/camel/pascal with one capture."))
			}
			args := n.Pipe.Cmds[0].Args
			if function, ok := args[0].(*parse.IdentifierNode); ok && function.Ident == "index" {
				valid := len(args) == 1
				if len(args) == 2 {
					width, ok := args[1].(*parse.NumberNode)
					valid = ok && len(width.Text) == 1 && width.Text[0] >= '1' && width.Text[0] <= '9'
				}
				if !valid {
					return invalid(lang.L("Use text, captures, or upper/lower/snake/kebab/camel/pascal with one capture."))
				}
				// Supply current row without mutable template state; repeated calls share its number.
				field := &parse.FieldNode{NodeType: parse.NodeField, Ident: []string{"Index"}}
				n.Pipe.Cmds[0].Args = append([]parse.Node{args[0], field}, args[1:]...)
				continue
			}
			if len(args) == 3 {
				function, functionOK := args[0].(*parse.IdentifierNode)
				format, formatOK := args[1].(*parse.StringNode)
				field, fieldOK := args[2].(*parse.FieldNode)
				if functionOK && function.Ident == "printf" && formatOK && regexp.MustCompile(`^%0[1-9]d$`).MatchString(format.Text) && fieldOK && len(field.Ident) == 1 && field.Ident[0] == "Index" {
					continue
				}
			}
			isHelper := len(args) == 2
			if len(args) == 2 {
				function, ok := args[0].(*parse.IdentifierNode)
				if !ok || helpers[function.Ident] == nil {
					return invalid(lang.L("Use text, captures, or upper/lower/snake/kebab/camel/pascal with one capture."))
				}
				args = args[1:]
			}
			if len(args) != 1 {
				return invalid(lang.L("Use text, captures, or upper/lower/snake/kebab/camel/pascal with one capture."))
			}
			field, ok := args[0].(*parse.FieldNode)
			if !ok || len(field.Ident) != 1 {
				return invalid(lang.L("Use text, captures, or upper/lower/snake/kebab/camel/pascal with one capture."))
			}
			if field.Ident[0] == "Index" && !isHelper {
				continue
			}
			if _, ok := names[field.Ident[0]]; !ok {
				return invalid(fmt.Sprintf(lang.L("Unknown capture: %s"), field.Ident[0]))
			}
		default:
			return invalid(lang.L("Use text, captures, or upper/lower/snake/kebab/camel/pascal with one capture."))
		}
	}
	return r, t, nil
}
