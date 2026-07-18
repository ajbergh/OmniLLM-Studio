package browser

import (
	"sync"
	"time"

	"github.com/go-rod/rod"
)

// Session is the in-memory state for a live browser page. opMu prevents two
// concurrent tool calls from driving the same page at once.
type Session struct {
	opMu sync.Mutex

	ID         string
	UserID     string
	Page       *rod.Page
	CreatedAt  time.Time
	LastUsedAt time.Time
	CurrentURL string
}
