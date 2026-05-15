package browser

import (
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const DefaultMaxExtractChars = 50000

// ExtractText returns visible body text with basic whitespace normalization.
func ExtractText(page *rod.Page, maxChars int) (string, error) {
	if maxChars <= 0 {
		maxChars = DefaultMaxExtractChars
	}
	el, err := page.Element("body")
	if err != nil {
		return "", err
	}
	text, err := el.Text()
	if err != nil {
		return "", err
	}
	text = strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	if len(text) > maxChars {
		text = text[:maxChars]
	}
	return text, nil
}

// ExtractHTML returns page HTML, capped to maxChars when configured.
func ExtractHTML(page *rod.Page, maxChars int) (string, error) {
	if maxChars <= 0 {
		maxChars = DefaultMaxExtractChars
	}
	html, err := page.HTML()
	if err != nil {
		return "", err
	}
	if len(html) > maxChars {
		html = html[:maxChars]
	}
	return html, nil
}

// TakeScreenshot captures either a page, full page, or selector screenshot.
func TakeScreenshot(page *rod.Page, selector string, fullPage bool) ([]byte, error) {
	if selector != "" {
		el, err := page.Element(selector)
		if err != nil {
			return nil, err
		}
		return el.Screenshot(proto.PageCaptureScreenshotFormatPng, 0)
	}
	return page.Screenshot(fullPage, &proto.PageCaptureScreenshot{
		Format: proto.PageCaptureScreenshotFormatPng,
	})
}

// SavePDF renders the current page to PDF bytes.
func SavePDF(page *rod.Page) ([]byte, error) {
	stream, err := page.PDF(&proto.PagePrintToPDF{
		PrintBackground: true,
	})
	if err != nil {
		return nil, err
	}
	defer stream.Close()
	return readAll(stream)
}

func detectBotProtection(text string) bool {
	normalized := strings.ToLower(text)
	signals := []string{
		"checking your browser",
		"cf-browser-verification",
		"enable javascript and cookies",
		"access denied",
		"attention required",
		"verify you are human",
		"captcha",
	}
	for _, signal := range signals {
		if strings.Contains(normalized, signal) {
			return true
		}
	}
	return false
}
