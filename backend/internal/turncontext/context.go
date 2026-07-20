package turncontext

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type ClientContext struct {
	Timezone string `json:"timezone,omitempty"`
	Locale   string `json:"locale,omitempty"`
	Country  string `json:"country,omitempty"`
	Region   string `json:"region,omitempty"`
	City     string `json:"city,omitempty"`
}

type TurnContext struct {
	ClientContext
	Now time.Time
}

type contextKey struct{}

func FromRequest(r *http.Request) ClientContext {
	if r == nil {
		return ClientContext{}
	}
	query := r.URL.Query()
	return normalize(ClientContext{
		Timezone: firstNonEmpty(query.Get("omnillm_timezone"), r.Header.Get("X-Omni-Timezone")),
		Locale:   firstNonEmpty(query.Get("omnillm_locale"), r.Header.Get("X-Omni-Locale")),
		Country:  firstNonEmpty(query.Get("omnillm_country"), r.Header.Get("X-Omni-Country")),
		Region:   firstNonEmpty(query.Get("omnillm_region"), r.Header.Get("X-Omni-Region")),
		City:     firstNonEmpty(query.Get("omnillm_city"), r.Header.Get("X-Omni-City")),
	})
}

func WithClientContext(ctx context.Context, client ClientContext) context.Context {
	client = normalize(client)
	now := time.Now()
	if client.Timezone != "" {
		if loc, err := time.LoadLocation(client.Timezone); err == nil {
			now = now.In(loc)
		} else {
			client.Timezone = ""
		}
	}
	return context.WithValue(ctx, contextKey{}, TurnContext{ClientContext: client, Now: now})
}

func AttachRequest(r *http.Request) *http.Request {
	if r == nil {
		return r
	}
	return r.WithContext(WithClientContext(r.Context(), FromRequest(r)))
}

func FromContext(ctx context.Context) TurnContext {
	if ctx != nil {
		if value, ok := ctx.Value(contextKey{}).(TurnContext); ok {
			if value.Now.IsZero() {
				value.Now = time.Now()
			}
			return value
		}
	}
	return TurnContext{ClientContext: ClientContext{Timezone: time.Local.String(), Locale: "en-US"}, Now: time.Now()}
}

func normalize(client ClientContext) ClientContext {
	client.Timezone = strings.TrimSpace(client.Timezone)
	client.Locale = strings.TrimSpace(client.Locale)
	client.Country = strings.ToUpper(strings.TrimSpace(client.Country))
	client.Region = strings.TrimSpace(client.Region)
	client.City = strings.TrimSpace(client.City)
	if client.Locale == "" {
		client.Locale = "en-US"
	}
	return client
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
