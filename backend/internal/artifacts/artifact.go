package artifacts

import "context"

// ArtifactFormat identifies the output file format.
type ArtifactFormat string

const (
	FormatDocx     ArtifactFormat = "docx"
	FormatXlsx     ArtifactFormat = "xlsx"
	FormatCsv      ArtifactFormat = "csv"
	FormatPdf      ArtifactFormat = "pdf"
	FormatMarkdown ArtifactFormat = "markdown"
	FormatHtml     ArtifactFormat = "html"
	FormatJson     ArtifactFormat = "json"
	FormatYaml     ArtifactFormat = "yaml"
)

// BlockType classifies a parsed content block.
type BlockType string

const (
	BlockHeading   BlockType = "heading"
	BlockParagraph BlockType = "paragraph"
	BlockBullet    BlockType = "bullet"
	BlockOrdered   BlockType = "ordered"
	BlockCode      BlockType = "code"
	BlockTable     BlockType = "table"
	BlockQuote     BlockType = "quote"
	BlockHR        BlockType = "hr"
)

// Artifact is the normalised representation of LLM output ready for rendering.
type Artifact struct {
	Title      string
	Subtitle   string
	Blocks     []Block
	Tables     []Table
	Metadata   map[string]string
	RawContent string // original LLM output, available to all renderers
}

// Block is a single structural unit inside an Artifact.
type Block struct {
	Type  BlockType
	Level int    // for BlockHeading: 1-6
	Text  string // for BlockParagraph, BlockHeading, BlockQuote
	Items []string
	Table *Table
	Code  *CodeBlock
}

// Table holds tabular data from a Markdown GFM table.
type Table struct {
	Name    string
	Headers []string
	Rows    [][]string
}

// CodeBlock holds a fenced code block.
type CodeBlock struct {
	Language string
	Content  string
}

// ArtifactRenderer converts an Artifact to a rendered file.
type ArtifactRenderer interface {
	Format() ArtifactFormat
	Render(ctx context.Context, artifact Artifact) (*RenderedArtifact, error)
}

// RenderedArtifact is the result of rendering — ready to write to disk or HTTP.
type RenderedArtifact struct {
	Filename    string
	ContentType string
	Bytes       []byte
}

// ContentTypeForFormat returns the MIME type for a given format.
func ContentTypeForFormat(f ArtifactFormat) string {
	switch f {
	case FormatXlsx:
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	case FormatCsv:
		return "text/csv; charset=utf-8"
	case FormatPdf:
		return "application/pdf"
	case FormatMarkdown:
		return "text/markdown; charset=utf-8"
	case FormatHtml:
		return "text/html; charset=utf-8"
	case FormatJson:
		return "application/json"
	case FormatYaml:
		return "application/yaml"
	case FormatDocx:
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	default:
		return "application/octet-stream"
	}
}

// ExtensionForFormat returns the file extension (without dot) for a format.
func ExtensionForFormat(f ArtifactFormat) string {
	switch f {
	case FormatXlsx:
		return "xlsx"
	case FormatCsv:
		return "csv"
	case FormatPdf:
		return "pdf"
	case FormatMarkdown:
		return "md"
	case FormatHtml:
		return "html"
	case FormatJson:
		return "json"
	case FormatYaml:
		return "yaml"
	case FormatDocx:
		return "docx"
	default:
		return "bin"
	}
}

// IconForFormat returns an emoji icon for chat display.
func IconForFormat(f ArtifactFormat) string {
	switch f {
	case FormatXlsx:
		return "📊"
	case FormatCsv:
		return "📋"
	case FormatPdf:
		return "📄"
	case FormatMarkdown:
		return "📝"
	case FormatHtml:
		return "🌐"
	case FormatJson:
		return "📦"
	case FormatYaml:
		return "⚙️"
	case FormatDocx:
		return "📄"
	default:
		return "📎"
	}
}
