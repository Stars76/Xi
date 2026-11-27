package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultGeoBaseURL      = "https://geocoding-api.open-meteo.com/v1/search"
	DefaultForecastBaseURL = "https://api.open-meteo.com/v1/forecast"
)

// GeoResult holds a resolved city coordinate pair.
type GeoResult struct {
	Name    string
	Country string
	Lat     float64
	Lon     float64
}

// CurrentResult holds the mapped weather state and temperature.
type CurrentResult struct {
	Weather   string // clear | rain | snow | storm | fog
	TempC     float64
	WMOCode   int
	FetchedAt time.Time
}

// Client queries Open-Meteo (free, zero API key) for optional live weather sync.
type Client struct {
	GeoBaseURL      string
	ForecastBaseURL string
	HTTPClient      *http.Client
}

func NewClient() *Client {
	return &Client{
		GeoBaseURL:      DefaultGeoBaseURL,
		ForecastBaseURL: DefaultForecastBaseURL,
		HTTPClient:      &http.Client{Timeout: 6 * time.Second},
	}
}

func (c *Client) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return &http.Client{Timeout: 6 * time.Second}
}

// MapWMOCode maps Open-Meteo WMO weather interpretation codes to the sea's weather states.
func MapWMOCode(code int) string {
	switch {
	case code >= 95:
		return "storm"
	case code == 45 || code == 48:
		return "fog"
	case (code >= 71 && code <= 77) || code == 85 || code == 86:
		return "snow"
	case (code >= 51 && code <= 67) || (code >= 80 && code <= 82):
		return "rain"
	default:
		return "clear"
	}
}

// WeatherLabel returns the concise Chinese display name for a weather mode.
func WeatherLabel(w string) string {
	switch w {
	case "rain":
		return "细雨"
	case "snow":
		return "飞雪"
	case "storm":
		return "雷雨"
	case "fog":
		return "薄雾"
	case "live":
		return "实况"
	default:
		return "晴空"
	}
}

// Geocode resolves a user-entered city name to coordinates via Open-Meteo Geocoding API.
func (c *Client) Geocode(ctx context.Context, city string) (GeoResult, error) {
	q := strings.TrimSpace(city)
	if q == "" {
		return GeoResult{}, errors.New("请输入城市名称")
	}
	base := DefaultGeoBaseURL
	if c != nil && c.GeoBaseURL != "" {
		base = c.GeoBaseURL
	}
	u, err := url.Parse(base)
	if err != nil {
		return GeoResult{}, err
	}
	params := u.Query()
	params.Set("name", q)
	params.Set("count", "1")
	params.Set("language", "zh")
	params.Set("format", "json")
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return GeoResult{}, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return GeoResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return GeoResult{}, fmt.Errorf("城市解析失败 (HTTP %d)", resp.StatusCode)
	}

	var payload struct {
		Results []struct {
			Name      string  `json:"name"`
			Admin1    string  `json:"admin1"`
			Country   string  `json:"country"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return GeoResult{}, err
	}
	if len(payload.Results) == 0 {
		return GeoResult{}, fmt.Errorf("未找到城市「%s」", q)
	}
	top := payload.Results[0]
	disp := top.Name
	if top.Admin1 != "" && top.Admin1 != top.Name {
		disp = top.Name + " · " + top.Admin1
	}
	return GeoResult{
		Name:    disp,
		Country: top.Country,
		Lat:     top.Latitude,
		Lon:     top.Longitude,
	}, nil
}

// FetchCurrent fetches the current weather code and temperature at (lat, lon).
func (c *Client) FetchCurrent(ctx context.Context, lat, lon float64) (CurrentResult, error) {
	base := DefaultForecastBaseURL
	if c != nil && c.ForecastBaseURL != "" {
		base = c.ForecastBaseURL
	}
	u, err := url.Parse(base)
	if err != nil {
		return CurrentResult{}, err
	}
	params := u.Query()
	params.Set("latitude", fmt.Sprintf("%.4f", lat))
	params.Set("longitude", fmt.Sprintf("%.4f", lon))
	params.Set("current", "weather_code,temperature_2m")
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return CurrentResult{}, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return CurrentResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return CurrentResult{}, fmt.Errorf("天气获取失败 (HTTP %d)", resp.StatusCode)
	}

	var payload struct {
		Current struct {
			WeatherCode   int     `json:"weather_code"`
			Temperature2m float64 `json:"temperature_2m"`
		} `json:"current"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return CurrentResult{}, err
	}
	return CurrentResult{
		Weather:   MapWMOCode(payload.Current.WeatherCode),
		TempC:     payload.Current.Temperature2m,
		WMOCode:   payload.Current.WeatherCode,
		FetchedAt: time.Now(),
	}, nil
}
