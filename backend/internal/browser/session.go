package browser

import (
	"sync"
	"time"

	"github.com/go-rod/rod"
)

// Session is the in-memory state for a live browser page. Every session owns an
// incognito browser context, preventing cookies, local storage, and cached
// credentials from crossing user/session boundaries.
type Session struct {
	opMu sync.Mutex

	ID             string
	UserID         string
	BrowserContext *rod.Browser
	Page           *rod.Page
	CreatedAt      time.Time
	LastUsedAt     time.Time
	CurrentURL     string
}
