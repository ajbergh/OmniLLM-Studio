package templates

import (
	"log"

	"github.com/ajbergh/omnillm-studio/internal/models"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

// builtInTemplates returns the default system templates shipped with the app.
func builtInTemplates() []repository.CreateTemplateInput {
	return []repository.CreateTemplateInput{
		{
			Name:        "Code Review",
			Description: "Review a code snippet for quality, bugs, and improvements.",
			Category:    "development",
			TemplateBody: `Please review the following {{language}} code.

Focus on:
- Potential bugs or logic errors
- Performance improvements
- Code style and readability
- Security concerns

{{code}}`,
			Variables: []models.TemplateVariable{
				{Name: "language", Label: "Language", Type: "text", Default: "Go", Required: true},
				{Name: "code", Label: "Code", Type: "text", Required: true},
			},
			IsSystem:  true,
			SortOrder: 1,
		},
		{
			Name:        "Bug Triage",
			Description: "Help triage and diagnose a reported bug.",
			Category:    "development",
			TemplateBody: `A bug has been reported with the following details:

**Summary:** {{summary}}
**Steps to reproduce:** {{steps}}
**Expected behavior:** {{expected}}
**Actual behavior:** {{actual}}

Please analyse this bug report. Suggest possible root causes and recommend debugging steps.`,
			Variables: []models.TemplateVariable{
				{Name: "summary", Label: "Summary", Type: "text", Required: true},
				{Name: "steps", Label: "Steps to Reproduce", Type: "text", Required: true},
				{Name: "expected", Label: "Expected Behavior", Type: "text", Required: true},
				{Name: "actual", Label: "Actual Behavior", Type: "text", Required: true},
			},
			IsSystem:  true,
			SortOrder: 2,
		},
		{
			Name:        "Architecture Review",
			Description: "Evaluate a system or module design for best practices.",
			Category:    "development",
			TemplateBody: `Please review the following architecture or design:

**Component:** {{component}}
**Description:** {{description}}
**Constraints:** {{constraints}}

Evaluate the design for:
- Separation of concerns
- Scalability
- Maintainability
- Potential single points of failure`,
			Variables: []models.TemplateVariable{
				{Name: "component", Label: "Component Name", Type: "text", Required: true},
				{Name: "description", Label: "Design Description", Type: "text", Required: true},
				{Name: "constraints", Label: "Known Constraints", Type: "text", Default: "None specified"},
			},
			IsSystem:  true,
			SortOrder: 3,
		},
		{
			Name:        "Summarize",
			Description: "Summarize a long piece of text into key points.",
			Category:    "general",
			TemplateBody: `Please provide a {{detail_level}} summary of the following text.

{{text}}`,
			Variables: []models.TemplateVariable{
				{Name: "detail_level", Label: "Detail Level", Type: "select", Default: "concise", Options: []string{"concise", "detailed", "bullet-points"}},
				{Name: "text", Label: "Text to Summarize", Type: "text", Required: true},
			},
			IsSystem:  true,
			SortOrder: 4,
		},
		{
			Name:        "Explain",
			Description: "Explain a concept or piece of code at a chosen level.",
			Category:    "general",
			TemplateBody: `Explain the following as if I am {{audience}}:

{{topic}}`,
			Variables: []models.TemplateVariable{
				{Name: "audience", Label: "Audience", Type: "select", Default: "a mid-level developer", Options: []string{"a beginner", "a mid-level developer", "an expert", "a non-technical person"}},
				{Name: "topic", Label: "Topic / Code", Type: "text", Required: true},
			},
			IsSystem:  true,
			SortOrder: 5,
		},
	}
}

// SeedDefaults inserts built-in prompt templates only if the prompt_templates
// table is empty.  Safe to call on every startup.
func SeedDefaults(repo *repository.TemplateRepo) {
	count, err := repo.Count()
	if err != nil {
		log.Printf("[templates] warning: could not check existing templates: %v", err)
		return
	}
	if count > 0 {
		return // already seeded or user has templates
	}

	defaults := builtInTemplates()
	log.Printf("[templates] seeding %d built-in templates", len(defaults))
	for _, t := range defaults {
		if _, err := repo.Create(t); err != nil {
			log.Printf("[templates] warning: failed to seed template %q: %v", t.Name, err)
		}
	}
}
