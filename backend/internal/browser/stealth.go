package browser

import (
	"math/rand"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"

var viewportProfiles = []struct {
	width  int
	height int
}{
	{1280, 800},
	{1366, 768},
	{1440, 900},
	{1920, 1080},
}

func applyStealthProfile(page *rod.Page) error {
	if page == nil {
		return nil
	}
	if err := page.SetUserAgent(&proto.NetworkSetUserAgentOverride{
		UserAgent:      defaultUserAgent,
		AcceptLanguage: "en-US,en;q=0.9",
	}); err != nil {
		return err
	}
	if _, err := page.SetExtraHeaders([]string{"Accept-Language", "en-US,en;q=0.9"}); err != nil {
		return err
	}

	profile := viewportProfiles[rand.Intn(len(viewportProfiles))]
	return page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width:             profile.width,
		Height:            profile.height,
		DeviceScaleFactor: 1,
		Mobile:            false,
	})
}
