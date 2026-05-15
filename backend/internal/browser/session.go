package browser

import (
	"time"

	"github.com/go-rod/rod"
)

// Session is the in-memory state for a live browser page.
type Session struct {
	ID         string
	UserID     string
	Page       *rod.Page
	CreatedAt  time.Time
	LastUsedAt time.Time
	CurrentURL string
}
