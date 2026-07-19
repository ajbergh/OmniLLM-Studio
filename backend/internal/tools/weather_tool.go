package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// WeatherLookupTool retrieves current conditions and a compact daily forecast
// from fixed Open-Meteo endpoints. User-controlled URLs are never accepted.
type WeatherLookupTool struct{ client *http.Client }

func NewWeatherLookupTool() *WeatherLookupTool {
	return &WeatherLookupTool{client: &http.Client{Timeout: 12 * time.Second}}
}

func (t *WeatherLookupTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "weather_lookup", Description: "Get current weather and a 1- to 7-day forecast for a named location or coordinates using Open-Meteo.",
		Category: "current_data", Enabled: true, Version: "1", Risk: RiskLow,
		ReadOnly: true, RequiresNetwork: true, SupportsParallel: true,
		DefaultTimeoutMS: 15000, MaxResultBytes: 65536,
		Parameters: json.RawMessage(`{
			"type":"object",
			"properties":{
				"location":{"type":"string","description":"City, region, or postal location"},
				"latitude":{"type":"number","minimum":-90,"maximum":90},
				"longitude":{"type":"number","minimum":-180,"maximum":180},
				"forecast_days":{"type":"integer","minimum":1,"maximum":7,"default":3},
				"temperature_unit":{"type":"string","enum":["celsius","fahrenheit"],"default":"celsius"},
				"wind_speed_unit":{"type":"string","enum":["kmh","mph","ms","kn"],"default":"kmh"}
			},
			"anyOf":[{"required":["location"]},{"required":["latitude","longitude"]}]
		}`),
		OutputSchema: json.RawMessage(`{"type":"object"}`),
	}
}

type weatherArgs struct {
	Location        string   `json:"location"`
	Latitude        *float64 `json:"latitude"`
	Longitude       *float64 `json:"longitude"`
	ForecastDays    int      `json:"forecast_days"`
	TemperatureUnit string   `json:"temperature_unit"`
	WindSpeedUnit   string   `json:"wind_speed_unit"`
}

type geocodeResponse struct {
	Results []struct {
		Name      string  `json:"name"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Country   string  `json:"country"`
		Admin1    string  `json:"admin1"`
		Timezone  string  `json:"timezone"`
	} `json:"results"`
}

type weatherResponse struct {
	Latitude     float64           `json:"latitude"`
	Longitude    float64           `json:"longitude"`
	Timezone     string            `json:"timezone"`
	CurrentUnits map[string]string `json:"current_units"`
	Current      struct {
		Time                string  `json:"time"`
		Temperature2M       float64 `json:"temperature_2m"`
		ApparentTemperature float64 `json:"apparent_temperature"`
		RelativeHumidity2M  float64 `json:"relative_humidity_2m"`
		Precipitation       float64 `json:"precipitation"`
		WeatherCode         int     `json:"weather_code"`
		WindSpeed10M        float64 `json:"wind_speed_10m"`
		WindGusts10M        float64 `json:"wind_gusts_10m"`
	} `json:"current"`
	DailyUnits map[string]string `json:"daily_units"`
	Daily      struct {
		Time                        []string  `json:"time"`
		WeatherCode                 []int     `json:"weather_code"`
		Temperature2MMax            []float64 `json:"temperature_2m_max"`
		Temperature2MMin            []float64 `json:"temperature_2m_min"`
		PrecipitationSum            []float64 `json:"precipitation_sum"`
		PrecipitationProbabilityMax []float64 `json:"precipitation_probability_max"`
		WindSpeed10MMax             []float64 `json:"wind_speed_10m_max"`
	} `json:"daily"`
}

func (t *WeatherLookupTool) Validate(raw json.RawMessage) error {
	var args weatherArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return err
	}
	if strings.TrimSpace(args.Location) == "" && (args.Latitude == nil || args.Longitude == nil) {
		return fmt.Errorf("location or latitude and longitude are required")
	}
	if args.Latitude != nil && (*args.Latitude < -90 || *args.Latitude > 90) {
		return fmt.Errorf("latitude must be between -90 and 90")
	}
	if args.Longitude != nil && (*args.Longitude < -180 || *args.Longitude > 180) {
		return fmt.Errorf("longitude must be between -180 and 180")
	}
	if args.ForecastDays < 0 || args.ForecastDays > 7 {
		return fmt.Errorf("forecast_days must be between 1 and 7")
	}
	return nil
}

func (t *WeatherLookupTool) Execute(ctx context.Context, raw json.RawMessage) (*ToolResult, error) {
	var args weatherArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, err
	}
	if args.ForecastDays == 0 {
		args.ForecastDays = 3
	}
	if args.TemperatureUnit == "" {
		args.TemperatureUnit = "celsius"
	}
	if args.WindSpeedUnit == "" {
		args.WindSpeedUnit = "kmh"
	}

	latitude, longitude := 0.0, 0.0
	displayName, timezone := strings.TrimSpace(args.Location), "auto"
	if args.Latitude != nil && args.Longitude != nil {
		latitude, longitude = *args.Latitude, *args.Longitude
		if displayName == "" {
			displayName = fmt.Sprintf("%.4f, %.4f", latitude, longitude)
		}
	} else {
		geocoded, err := t.geocode(ctx, args.Location)
		if err != nil {
			return nil, err
		}
		latitude, longitude = geocoded.Latitude, geocoded.Longitude
		timezone = geocoded.Timezone
		displayName = geocoded.Name
		if geocoded.Admin1 != "" {
			displayName += ", " + geocoded.Admin1
		}
		if geocoded.Country != "" {
			displayName += ", " + geocoded.Country
		}
	}

	query := url.Values{}
	query.Set("latitude", strconv.FormatFloat(latitude, 'f', 6, 64))
	query.Set("longitude", strconv.FormatFloat(longitude, 'f', 6, 64))
	query.Set("current", "temperature_2m,apparent_temperature,relative_humidity_2m,precipitation,weather_code,wind_speed_10m,wind_gusts_10m")
	query.Set("daily", "weather_code,temperature_2m_max,temperature_2m_min,precipitation_sum,precipitation_probability_max,wind_speed_10m_max")
	query.Set("forecast_days", strconv.Itoa(args.ForecastDays))
	query.Set("temperature_unit", args.TemperatureUnit)
	query.Set("wind_speed_unit", args.WindSpeedUnit)
	query.Set("timezone", timezone)
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.open-meteo.com/v1/forecast?"+query.Encode(), nil)
	request.Header.Set("User-Agent", "OmniLLM-Studio/0.2 weather tool")
	response, err := t.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("weather request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather service returned HTTP %d", response.StatusCode)
	}
	var forecast weatherResponse
	if err := json.NewDecoder(http.MaxBytesReader(nil, response.Body, 2<<20)).Decode(&forecast); err != nil {
		return nil, fmt.Errorf("decode weather response: %w", err)
	}
	structured, _ := json.Marshal(map[string]interface{}{
		"location": displayName, "latitude": latitude, "longitude": longitude,
		"timezone": forecast.Timezone, "current": forecast.Current,
		"current_units": forecast.CurrentUnits, "daily": forecast.Daily, "daily_units": forecast.DailyUnits,
		"source": "Open-Meteo", "retrieved_at": time.Now().UTC().Format(time.RFC3339),
	})
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("%s: %s, %.1f%s (feels %.1f%s), humidity %.0f%%, wind %.1f%s.\n",
		displayName, weatherCodeDescription(forecast.Current.WeatherCode),
		forecast.Current.Temperature2M, forecast.CurrentUnits["temperature_2m"],
		forecast.Current.ApparentTemperature, forecast.CurrentUnits["apparent_temperature"],
		forecast.Current.RelativeHumidity2M, forecast.Current.WindSpeed10M, forecast.CurrentUnits["wind_speed_10m"]))
	for index, date := range forecast.Daily.Time {
		if index >= len(forecast.Daily.WeatherCode) || index >= len(forecast.Daily.Temperature2MMax) || index >= len(forecast.Daily.Temperature2MMin) {
			break
		}
		precipitationProbability := 0.0
		if index < len(forecast.Daily.PrecipitationProbabilityMax) {
			precipitationProbability = forecast.Daily.PrecipitationProbabilityMax[index]
		}
		builder.WriteString(fmt.Sprintf("- %s: %s, high %.1f%s, low %.1f%s, precipitation chance %.0f%%\n",
			date, weatherCodeDescription(forecast.Daily.WeatherCode[index]),
			forecast.Daily.Temperature2MMax[index], forecast.DailyUnits["temperature_2m_max"],
			forecast.Daily.Temperature2MMin[index], forecast.DailyUnits["temperature_2m_min"], precipitationProbability))
	}
	return &ToolResult{Content: strings.TrimSpace(builder.String()), Structured: structured, Metadata: map[string]interface{}{"source": "Open-Meteo", "location": displayName}}, nil
}

func (t *WeatherLookupTool) geocode(ctx context.Context, location string) (struct {
	Name      string
	Latitude  float64
	Longitude float64
	Country   string
	Admin1    string
	Timezone  string
}, error) {
	var result struct {
		Name      string
		Latitude  float64
		Longitude float64
		Country   string
		Admin1    string
		Timezone  string
	}
	query := url.Values{"name": []string{location}, "count": []string{"1"}, "language": []string{"en"}, "format": []string{"json"}}
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://geocoding-api.open-meteo.com/v1/search?"+query.Encode(), nil)
	request.Header.Set("User-Agent", "OmniLLM-Studio/0.2 weather tool")
	response, err := t.client.Do(request)
	if err != nil {
		return result, fmt.Errorf("weather geocoding: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf("weather geocoding returned HTTP %d", response.StatusCode)
	}
	var decoded geocodeResponse
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&decoded); err != nil {
		return result, fmt.Errorf("decode weather geocoding: %w", err)
	}
	if len(decoded.Results) == 0 {
		return result, fmt.Errorf("location not found: %s", location)
	}
	match := decoded.Results[0]
	result.Name, result.Latitude, result.Longitude = match.Name, match.Latitude, match.Longitude
	result.Country, result.Admin1, result.Timezone = match.Country, match.Admin1, match.Timezone
	return result, nil
}

func weatherCodeDescription(code int) string {
	descriptions := map[int]string{
		0: "clear sky", 1: "mainly clear", 2: "partly cloudy", 3: "overcast",
		45: "fog", 48: "depositing rime fog", 51: "light drizzle", 53: "drizzle", 55: "dense drizzle",
		56: "light freezing drizzle", 57: "freezing drizzle", 61: "light rain", 63: "rain", 65: "heavy rain",
		66: "light freezing rain", 67: "freezing rain", 71: "light snow", 73: "snow", 75: "heavy snow",
		77: "snow grains", 80: "light rain showers", 81: "rain showers", 82: "violent rain showers",
		85: "light snow showers", 86: "heavy snow showers", 95: "thunderstorm", 96: "thunderstorm with light hail", 99: "thunderstorm with heavy hail",
	}
	if description, ok := descriptions[code]; ok {
		return description
	}
	return fmt.Sprintf("weather code %d", code)
}
