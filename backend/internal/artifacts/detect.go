package artifacts

import "strings"

// DetectFormat inspects a user message for an explicit artifact export request.
// It returns the detected format and true, or ("", false) if no request is found.
// Word-document (.docx) intent is handled separately by the existing word-doc pipeline.
func DetectFormat(msg string) (ArtifactFormat, bool) {
	lower := strings.ToLower(msg)

	// Excel / XLSX — checked before CSV to avoid "importable into Excel" matching CSV first
	for _, kw := range xlsxKeywords {
		if strings.Contains(lower, kw) {
			return FormatXlsx, true
		}
	}
	// CSV
	for _, kw := range csvKeywords {
		if strings.Contains(lower, kw) {
			return FormatCsv, true
		}
	}
	// PDF
	for _, kw := range pdfKeywords {
		if strings.Contains(lower, kw) {
			return FormatPdf, true
		}
	}
	// Markdown
	for _, kw := range markdownKeywords {
		if strings.Contains(lower, kw) {
			return FormatMarkdown, true
		}
	}
	// HTML
	for _, kw := range htmlKeywords {
		if strings.Contains(lower, kw) {
			return FormatHtml, true
		}
	}
	// JSON
	for _, kw := range jsonKeywords {
		if strings.Contains(lower, kw) {
			return FormatJson, true
		}
	}
	// YAML
	for _, kw := range yamlKeywords {
		if strings.Contains(lower, kw) {
			return FormatYaml, true
		}
	}

	return "", false
}

var xlsxKeywords = []string{
	"as excel", "in excel", "to excel", "excel file", "excel spreadsheet",
	"export as xlsx", "export to xlsx", "as xlsx", ".xlsx",
	"create a spreadsheet", "make a spreadsheet", "turn this into a spreadsheet",
	"turn this table into a workbook", "make this a workbook",
	"create a tracker", "make a tracker",
	"put this in excel", "put it in excel",
	"make this an excel",
}

var csvKeywords = []string{
	"export as csv", "export to csv", "as csv", ".csv", "csv file",
	"give me a csv", "in csv", "to csv",
	"comma-separated", "comma separated",
	"make this importable into excel",
}

var pdfKeywords = []string{
	"as pdf", "as a pdf", "export as pdf", "export to pdf", ".pdf", "pdf file",
	"make this a pdf", "create a pdf", "generate a pdf", "turn this into a pdf",
	"create a printable report", "make a printable report",
	"printable version", "make this client-ready",
	"one-page pdf", "one page pdf",
}

var markdownKeywords = []string{
	"as markdown", "export as markdown", "save as markdown", "export to markdown",
	"as md", "save as md", ".md file", "give me a markdown",
	"make this a readme", "save as readme",
	"as a markdown file", "markdown file",
}

var htmlKeywords = []string{
	"as html", "export as html", "save as html", "export to html",
	".html file", "html file", "html report",
	"make this a web page", "make this a webpage",
	"create a standalone html", "standalone html",
	"render as html",
}

var jsonKeywords = []string{
	"as json", "return as json", "export as json", "save as json",
	".json file", "json file",
	"make this an api payload", "api payload",
	"structured json", "as a json",
	"output as json", "give me json",
}

var yamlKeywords = []string{
	"as yaml", "return as yaml", "export as yaml", "save as yaml",
	".yaml file", ".yml file", "yaml file", "yml file",
	"make this a config file", "as a config file",
	"kubernetes yaml", "k8s yaml", "helm values",
	"output as yaml", "give me yaml",
}

// ArtifactSystemDirective returns a format-specific instruction to include in
// the system prompt when an artifact export is requested.
func ArtifactSystemDirective(f ArtifactFormat) string {
	switch f {
	case FormatXlsx:
		return `EXCEL MODE: The user is requesting an Excel spreadsheet. Structure your response as clean Markdown tables. Each distinct dataset should have its own table with a clear ## heading above it. Use simple column headers. Keep numeric values as plain numbers (no % signs, currency symbols, or formatting). The application will automatically convert your response to a downloadable .xlsx file — do NOT say you cannot create Excel files.`
	case FormatCsv:
		return `CSV MODE: The user is requesting a CSV file. Provide a single clean Markdown table with clear column headers and well-structured data rows. Keep numeric values as plain numbers. The application will automatically export this as a downloadable .csv file — do NOT say you cannot create CSV files.`
	case FormatPdf:
		return `PDF MODE: The user is requesting a PDF document. Structure your response as a well-organized document with a clear title (# heading), section headings (##, ###), paragraphs, bullet lists, and simple tables where appropriate. Keep it concise and printable. The application will automatically generate a downloadable .pdf file — do NOT say you cannot create PDF files.`
	case FormatMarkdown:
		return `MARKDOWN MODE: The user is requesting a Markdown file. Write clean, well-structured GitHub-Flavored Markdown. The application will save this directly as a downloadable .md file — do NOT say you cannot create Markdown files.`
	case FormatHtml:
		return `HTML MODE: The user is requesting an HTML page. Write your response as clean Markdown — the application will convert it to a standalone HTML page. Do NOT say you cannot create HTML files.`
	case FormatJson:
		return `JSON MODE: The user is requesting a JSON file. Output ONLY valid JSON (no Markdown fences, no explanatory prose, no code block delimiters). The application will save this directly as a downloadable .json file — do NOT say you cannot create JSON files.`
	case FormatYaml:
		return `YAML MODE: The user is requesting a YAML file. Output ONLY valid YAML (no Markdown fences, no explanatory prose, no code block delimiters). The application will save this directly as a downloadable .yaml file — do NOT say you cannot create YAML files.`
	default:
		return ""
	}
}

// ArtifactCapabilityDirective is always appended to the system prompt so the
// assistant never says it cannot produce local files.
const ArtifactCapabilityDirective = `This application (OmniLLM-Studio) generates downloadable files locally. When asked for any of the following formats, say you CAN create it — never say you are unable to generate files or that you can only provide text to copy:
- Word document (.docx)
- Excel spreadsheet (.xlsx)
- CSV (.csv)
- PDF (.pdf)
- Markdown (.md)
- HTML page (.html)
- JSON (.json)
- YAML (.yaml)
The application handles file creation automatically from your response.`
