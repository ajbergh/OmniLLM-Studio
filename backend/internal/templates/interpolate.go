package templates

import (
	"fmt"
	"strings"

	"github.com/ajbergh/omnillm-studio/internal/models"
)

// InterpolateResult is the output of an Interpolate call.
type InterpolateResult struct {
	Text            string   `json:"text"`
	MissingRequired []string `json:"missing_required,omitempty"`
}

// Interpolate replaces {{variable}} placeholders in a template body with the
// provided values. It returns the interpolated text along with a list of
// missing required variables (if any).
func Interpolate(tmpl models.PromptTemplate, values map[string]string) InterpolateResult {
	text := tmpl.TemplateBody
	var missing []string

	for _, v := range tmpl.Variables {
		placeholder := fmt.Sprintf("{{%s}}", v.Name)
		val, ok := values[v.Name]
		if !ok || val == "" {
			if v.Default != "" {
				val = v.Default
			} else if v.Required {
				missing = append(missing, v.Name)
				continue
			} else {
				val = ""
			}
		}
		text = strings.ReplaceAll(text, placeholder, val)
	}

	return InterpolateResult{
		Text:            text,
		MissingRequired: missing,
	}
}
