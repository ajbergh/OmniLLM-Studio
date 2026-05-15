package api

import (
	"context"
	"fmt"

	"github.com/ajbergh/omnillm-studio/internal/browser"
	"github.com/ajbergh/omnillm-studio/internal/repository"
)

type browserFallbackNavigator struct {
	mgr   *browser.Manager
	flags *repository.FeatureFlagRepo
}

func newBrowserFallbackNavigator(mgr *browser.Manager, flags *repository.FeatureFlagRepo) *browserFallbackNavigator {
	return &browserFallbackNavigator{mgr: mgr, flags: flags}
}

func (n *browserFallbackNavigator) Navigate(ctx context.Context, url string) (string, string, error) {
	if n == nil || n.mgr == nil {
		return "", "", fmt.Errorf("browser manager unavailable")
	}
	if n.flags != nil && !n.flags.IsEnabled("headless_browser") {
		return "", "", fmt.Errorf("headless browser feature flag is disabled")
	}
	return n.mgr.Navigate(ctx, url)
}
