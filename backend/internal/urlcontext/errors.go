package urlcontext

import "errors"

var (
	ErrNoURLDetected       = errors.New("no url detected")
	ErrURLContextNotNeeded = errors.New("url context not needed")
	ErrUnsupportedScheme   = errors.New("unsupported url scheme")
	ErrBlockedHost         = errors.New("blocked host: private/internal network")
	ErrFetchTimeout        = errors.New("url fetch timeout")
	ErrContentTooLarge     = errors.New("content too large")
	ErrUnsupportedContent  = errors.New("unsupported content type")
	ErrGitHubRateLimited   = errors.New("github api rate limited")
	ErrGitHubPrivate       = errors.New("github repository private or unavailable")
	ErrInsufficientContent = errors.New("insufficient readable content extracted")
	// ErrNoUsableContent is returned at the aggregate level when ALL URL fetches
	// failed or produced no usable content. Unlike ErrInsufficientContent (which
	// is a soft per-URL error), this error bypasses the LLM entirely via
	// IsRequiredContextError so the model cannot hallucinate about the URL.
	ErrNoUsableContent = errors.New("no usable content could be extracted from any provided URL")
)

// IsRequiredContextError returns true when URL context was needed but could not be resolved.
// In that case the caller should NOT fall through to normal LLM memory.
func IsRequiredContextError(err error) bool {
	return errors.Is(err, ErrBlockedHost) ||
		errors.Is(err, ErrUnsupportedScheme) ||
		errors.Is(err, ErrGitHubPrivate) ||
		errors.Is(err, ErrGitHubRateLimited) ||
		errors.Is(err, ErrNoUsableContent)
}

// UserFacingErrorMessage returns a safe message suitable for sending to the user.
func UserFacingErrorMessage(err error) string {
	switch {
	case errors.Is(err, ErrUnsupportedScheme):
		return "I detected a URL in your message, but only http:// and https:// URLs are supported. Please check the URL and try again."
	case errors.Is(err, ErrBlockedHost):
		return "I detected a URL pointing to a private or internal network address. For security reasons, I cannot fetch those URLs. Please use a publicly accessible URL."
	case errors.Is(err, ErrFetchTimeout):
		return "I tried to read the URL you provided, but the request timed out. Please check that the URL is publicly accessible and try again."
	case errors.Is(err, ErrContentTooLarge):
		return "The content at the provided URL is too large to process directly. I will use the portion I was able to retrieve."
	case errors.Is(err, ErrGitHubRateLimited):
		return "The GitHub API rate limit was reached while trying to inspect the repository. Please try again in a few minutes, or configure a GITHUB_CONTEXT_TOKEN for higher limits."
	case errors.Is(err, ErrGitHubPrivate):
		return "The GitHub repository could not be accessed. It may be private or unavailable. Please ensure the repository is public, or configure a GITHUB_CONTEXT_TOKEN with appropriate access."
	case errors.Is(err, ErrInsufficientContent):
		return "I could not extract enough readable content from the provided URL. The page may require JavaScript rendering or the content may be behind authentication."
	case errors.Is(err, ErrNoUsableContent):
		return "I tried to read the URL(s) you provided but could not extract useful content. This typically happens when a page requires JavaScript to render (like news homepages and social media). Please copy and paste the text you want me to analyze directly into the chat."
	default:
		return "I detected a URL in your message but could not fetch its content. Please check that the URL is publicly accessible and try again."
	}
}
