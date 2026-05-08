package news

import (
	"time"
)

// SourceName is the display name for the Actually Relevant News API.
const SourceName = "Actually Relevant News"

// NewsIntentType describes the kind of news response the user wants.
type NewsIntentType string

const (
	NewsIntentUnknown   NewsIntentType = "unknown"
	NewsIntentFrontPage NewsIntentType = "front_page"
	NewsIntentBrief     NewsIntentType = "brief"
	NewsIntentDetailed  NewsIntentType = "detailed"
)

// NewsIntent is the result of intent detection for a user prompt.
type NewsIntent struct {
	Handled        bool
	Confidence     float64
	Query          string
	IssueSlug      string
	Search         string
	PageSize       int
	WantsHTML      bool
	WantsBrief     bool
	WantsDetailed  bool
	WantsFrontPage bool
	IntentType     NewsIntentType
	Reason         string
}

// StoryPage is the paginated response from the Actually Relevant API.
type StoryPage struct {
	Data       []Story `json:"data"`
	Total      int     `json:"total"`
	Page       int     `json:"page"`
	PageSize   int     `json:"pageSize"`
	TotalPages int     `json:"totalPages"`
}

// Story is a single news story from the Actually Relevant API.
type Story struct {
	ID               string     `json:"id"`
	Slug             string     `json:"slug"`
	SourceURL        string     `json:"sourceUrl"`
	SourceTitle      string     `json:"sourceTitle"`
	Title            string     `json:"title"`
	TitleLabel       string     `json:"titleLabel"`
	DateCrawled      *time.Time `json:"dateCrawled"`
	DatePublished    *time.Time `json:"datePublished"`
	Status           string     `json:"status"`
	RelevancePre     *int       `json:"relevancePre"`
	Relevance        *int       `json:"relevance"`
	EmotionTag       string     `json:"emotionTag"`
	Summary          string     `json:"summary"`
	Quote            string     `json:"quote"`
	QuoteAttribution string     `json:"quoteAttribution"`
	MarketingBlurb   string     `json:"marketingBlurb"`
	RelevanceReasons string     `json:"relevanceReasons"`
	RelevanceSummary string     `json:"relevanceSummary"`
	Antifactors      string     `json:"antifactors"`
	Issue            *IssueRef  `json:"issue"`
	Feed             *FeedRef   `json:"feed"`
}

// IssueRef is a reference to an issue area.
type IssueRef struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// FeedRef is a reference to a feed source.
type FeedRef struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	DisplayTitle string    `json:"displayTitle"`
	Issue        *IssueRef `json:"issue"`
}

// Issue represents a news issue area from the API.
type Issue struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// ClusterResponse is the response from the cluster endpoint.
type ClusterResponse struct {
	Stories []Story `json:"stories"`
}

// StoryQuery holds parameters for querying the Actually Relevant API.
type StoryQuery struct {
	Page        int
	PageSize    int
	IssueSlug   string
	Search      string
	EmotionTags []string
}

// LookupResult is the result of a news lookup attempt.
type LookupResult struct {
	Handled  bool
	Content  string
	Metadata map[string]any
}

// EditionInput holds data for formatting a newspaper edition.
type EditionInput struct {
	Prompt      string
	Intent      NewsIntent
	Stories     []Story
	Total       int
	Broadened   bool
	GeneratedAt time.Time
}
