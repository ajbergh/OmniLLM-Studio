package tools

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// CurrencyConvertTool converts currencies using the ECB's euro reference rates.
type CurrencyConvertTool struct {
	client *http.Client
	mu     sync.Mutex
	cache  map[string]float64
	date   string
	loaded time.Time
}

func NewCurrencyConvertTool() *CurrencyConvertTool {
	return &CurrencyConvertTool{client: &http.Client{Timeout: 12 * time.Second}}
}

func (t *CurrencyConvertTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "currency_convert", Description: "Convert an amount using the European Central Bank's latest published euro foreign-exchange reference rates.",
		Category: "current_data", Enabled: true, Version: "1", Risk: RiskLow,
		ReadOnly: true, RequiresNetwork: true, SupportsParallel: true,
		DefaultTimeoutMS: 15000, MaxResultBytes: 32768,
		Parameters: json.RawMessage(`{
			"type":"object","required":["amount","from","to"],
			"properties":{
				"amount":{"type":"number"},
				"from":{"type":"string","minLength":3,"maxLength":3},
				"to":{"type":"string","minLength":3,"maxLength":3},
				"precision":{"type":"integer","minimum":0,"maximum":8,"default":4}
			}
		}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	}
}

type currencyArgs struct {
	Amount    float64 `json:"amount"`
	From      string  `json:"from"`
	To        string  `json:"to"`
	Precision int     `json:"precision"`
}

type ecbEnvelope struct {
	Cube struct {
		Cube struct {
			Time  string `xml:"time,attr"`
			Rates []struct {
				Currency string  `xml:"currency,attr"`
				Rate     float64 `xml:"rate,attr"`
			} `xml:"Cube"`
		} `xml:"Cube"`
	} `xml:"Cube"`
}

func (t *CurrencyConvertTool) Validate(raw json.RawMessage) error {
	var args currencyArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	args.From = strings.ToUpper(strings.TrimSpace(args.From))
	args.To = strings.ToUpper(strings.TrimSpace(args.To))
	if len(args.From) != 3 || len(args.To) != 3 {
		return fmt.Errorf("from and to must be three-letter currency codes")
	}
	if args.Precision < 0 || args.Precision > 8 {
		return fmt.Errorf("precision must be between 0 and 8")
	}
	return nil
}

func (t *CurrencyConvertTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var args currencyArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	args.From = strings.ToUpper(strings.TrimSpace(args.From))
	args.To = strings.ToUpper(strings.TrimSpace(args.To))
	if args.Precision == 0 {
		args.Precision = 4
	}
	rates, date, err := t.rates(ctx)
	if err != nil {
		return nil, err
	}
	fromRate, ok := rates[args.From]
	if !ok {
		return nil, fmt.Errorf("ECB reference rate is unavailable for %s", args.From)
	}
	toRate, ok := rates[args.To]
	if !ok {
		return nil, fmt.Errorf("ECB reference rate is unavailable for %s", args.To)
	}
	result := args.Amount / fromRate * toRate
	structured, _ := json.Marshal(map[string]interface{}{
		"amount": args.Amount, "from": args.From, "to": args.To, "result": result,
		"rate": toRate / fromRate, "reference_date": date,
		"source": "European Central Bank", "retrieved_at": time.Now().UTC().Format(time.RFC3339),
	})
	content := fmt.Sprintf("%.*f %s = %.*f %s using ECB reference rates published %s.", args.Precision, args.Amount, args.From, args.Precision, result, args.To, date)
	return &ToolResult{Content: content, Structured: structured, Metadata: map[string]interface{}{"source": "ECB", "reference_date": date}}, nil
}

func (t *CurrencyConvertTool) rates(ctx context.Context) (map[string]float64, string, error) {
	t.mu.Lock()
	if len(t.cache) > 0 && time.Since(t.loaded) < 6*time.Hour {
		rates := cloneRates(t.cache)
		date := t.date
		t.mu.Unlock()
		return rates, date, nil
	}
	t.mu.Unlock()

	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml", nil)
	request.Header.Set("User-Agent", "OmniLLM-Studio/0.2 currency tool")
	response, err := t.client.Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("ECB exchange-rate request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("ECB exchange-rate service returned HTTP %d", response.StatusCode)
	}
	var envelope ecbEnvelope
	decoder := xml.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&envelope); err != nil {
		return nil, "", fmt.Errorf("decode ECB exchange rates: %w", err)
	}
	rates := map[string]float64{"EUR": 1}
	for _, item := range envelope.Cube.Cube.Rates {
		if item.Currency != "" && item.Rate > 0 {
			rates[strings.ToUpper(item.Currency)] = item.Rate
		}
	}
	if len(rates) <= 1 || envelope.Cube.Cube.Time == "" {
		return nil, "", fmt.Errorf("ECB exchange-rate response contained no rates")
	}
	t.mu.Lock()
	t.cache = cloneRates(rates)
	t.date = envelope.Cube.Cube.Time
	t.loaded = time.Now()
	t.mu.Unlock()
	return rates, envelope.Cube.Cube.Time, nil
}

func cloneRates(source map[string]float64) map[string]float64 {
	copy := make(map[string]float64, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}
