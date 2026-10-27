package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// The weather tool turns a place ("Joplin, MO", "90210", "Mount Rainier") into
// coordinates with OpenStreetMap's Nominatim geocoder, then asks the National
// Weather Service for that point's forecast and the nearest station's latest
// observation. Both services are free and need no key, but both ask callers to
// send an identifying User-Agent. Nominatim allows about one request a second.
var (
	geocodeURL    = "https://nominatim.openstreetmap.org/search"
	weatherAPIURL = "https://api.weather.gov"
	weatherHTTP   = &http.Client{Timeout: 15 * time.Second}
)

// WeatherUserAgent identifies this app to Nominatim and the NWS. Both ask for
// contact details (a URL or email) in production; set WEATHER_USER_AGENT.
var WeatherUserAgent = "professor-mac-chat/0.1 (prototype)"

// How many forecast periods to return. NWS periods are half days ("Today",
// "Tonight", "Wednesday", ...), so 4 covers about the next two days.
const forecastPeriods = 4

type weatherReport struct {
	Location    string           `json:"location"`     // what the geocoder matched
	NWSLocation string           `json:"nws_location"` // nearest place NWS names, e.g. "Joplin, MO"
	Latitude    float64          `json:"latitude"`
	Longitude   float64          `json:"longitude"`
	TimeZone    string           `json:"time_zone"`
	Current     *currentWeather  `json:"current_conditions,omitempty"`
	Forecast    []forecastPeriod `json:"forecast"`
	Source      string           `json:"source"`
}

type currentWeather struct {
	Station      string   `json:"station"`
	ObservedAt   string   `json:"observed_at"`
	Description  string   `json:"description"`
	TemperatureF *float64 `json:"temperature_f"`
	HumidityPct  *float64 `json:"relative_humidity_pct"`
	WindMPH      *float64 `json:"wind_mph"`
}

type forecastPeriod struct {
	Name                  string   `json:"name"` // "Today", "Tonight", "Wednesday"
	Start                 string   `json:"start"`
	Temperature           float64  `json:"temperature"`
	TemperatureUnit       string   `json:"temperature_unit"`
	Wind                  string   `json:"wind"`
	ChanceOfPrecipitation *float64 `json:"chance_of_precipitation_pct"`
	ShortForecast         string   `json:"short_forecast"`
	DetailedForecast      string   `json:"detailed_forecast"`
}

func getWeatherForecast(ctx context.Context, location string) (weatherReport, error) {
	if strings.TrimSpace(location) == "" {
		return weatherReport{}, fmt.Errorf("location is required")
	}
	place, err := geocode(ctx, location)
	if err != nil {
		return weatherReport{}, err
	}

	// NWS wants at most 4 decimal places and redirects otherwise.
	coords := formatCoord(place.lat) + "," + formatCoord(place.lon)
	var point struct {
		Properties struct {
			Forecast            string `json:"forecast"`
			ObservationStations string `json:"observationStations"`
			TimeZone            string `json:"timeZone"`
			RelativeLocation    struct {
				Properties struct {
					City  string `json:"city"`
					State string `json:"state"`
				} `json:"properties"`
			} `json:"relativeLocation"`
		} `json:"properties"`
	}
	if err := getJSON(ctx, weatherAPIURL+"/points/"+coords, &point); err != nil {
		return weatherReport{}, fmt.Errorf("NWS has no forecast for %s (%s): %w", place.name, coords, err)
	}
	p := point.Properties
	if p.Forecast == "" {
		return weatherReport{}, fmt.Errorf("NWS returned no forecast link for %s", place.name)
	}

	var forecast struct {
		Properties struct {
			Periods []struct {
				Name                       string  `json:"name"`
				StartTime                  string  `json:"startTime"`
				Temperature                float64 `json:"temperature"`
				TemperatureUnit            string  `json:"temperatureUnit"`
				WindSpeed                  string  `json:"windSpeed"`
				WindDirection              string  `json:"windDirection"`
				ShortForecast              string  `json:"shortForecast"`
				DetailedForecast           string  `json:"detailedForecast"`
				ProbabilityOfPrecipitation struct {
					Value *float64 `json:"value"`
				} `json:"probabilityOfPrecipitation"`
			} `json:"periods"`
		} `json:"properties"`
	}
	if err := getJSON(ctx, p.Forecast, &forecast); err != nil {
		return weatherReport{}, fmt.Errorf("getting the NWS forecast: %w", err)
	}

	report := weatherReport{
		Location:    place.name,
		NWSLocation: strings.Trim(p.RelativeLocation.Properties.City+", "+p.RelativeLocation.Properties.State, ", "),
		Latitude:    place.lat,
		Longitude:   place.lon,
		TimeZone:    p.TimeZone,
		Forecast:    []forecastPeriod{},
		Source:      "National Weather Service (api.weather.gov); location lookup © OpenStreetMap contributors",
	}
	for i, period := range forecast.Properties.Periods {
		if i == forecastPeriods {
			break
		}
		report.Forecast = append(report.Forecast, forecastPeriod{
			Name:                  period.Name,
			Start:                 period.StartTime,
			Temperature:           period.Temperature,
			TemperatureUnit:       period.TemperatureUnit,
			Wind:                  strings.TrimSpace(period.WindSpeed + " " + period.WindDirection),
			ChanceOfPrecipitation: period.ProbabilityOfPrecipitation.Value,
			ShortForecast:         period.ShortForecast,
			DetailedForecast:      period.DetailedForecast,
		})
	}

	// Current conditions are a bonus: if the station lookup fails, the
	// forecast alone still answers the question.
	if p.ObservationStations != "" {
		report.Current, _ = currentConditions(ctx, p.ObservationStations)
	}
	return report, nil
}

type geoPlace struct {
	name     string
	lat, lon float64
}

// geocode finds a US place by free text: city, "city, state", ZIP code,
// address, or landmark.
func geocode(ctx context.Context, location string) (geoPlace, error) {
	q := url.Values{
		"q":            {location},
		"format":       {"jsonv2"},
		"countrycodes": {"us"},
		"limit":        {"1"},
	}
	var results []struct {
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		DisplayName string `json:"display_name"`
	}
	if err := getJSON(ctx, geocodeURL+"?"+q.Encode(), &results); err != nil {
		return geoPlace{}, fmt.Errorf("looking up %q: %w", location, err)
	}
	if len(results) == 0 {
		return geoPlace{}, fmt.Errorf("couldn't find a US location matching %q; the NWS only covers the US", location)
	}
	lat, err1 := strconv.ParseFloat(results[0].Lat, 64)
	lon, err2 := strconv.ParseFloat(results[0].Lon, 64)
	if err1 != nil || err2 != nil {
		return geoPlace{}, fmt.Errorf("the geocoder returned bad coordinates for %q", location)
	}
	return geoPlace{name: results[0].DisplayName, lat: lat, lon: lon}, nil
}

// currentConditions returns the latest observation from the first (nearest)
// station in an NWS observationStations list.
func currentConditions(ctx context.Context, stationsURL string) (*currentWeather, error) {
	var stations struct {
		Features []struct {
			ID         string `json:"id"` // the station's URL
			Properties struct {
				Name string `json:"name"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := getJSON(ctx, stationsURL, &stations); err != nil {
		return nil, err
	}
	if len(stations.Features) == 0 {
		return nil, fmt.Errorf("no observation stations")
	}
	station := stations.Features[0]

	type measurement struct {
		Value *float64 `json:"value"`
	}
	var obs struct {
		Properties struct {
			Timestamp        string      `json:"timestamp"`
			TextDescription  string      `json:"textDescription"`
			Temperature      measurement `json:"temperature"`      // °C
			RelativeHumidity measurement `json:"relativeHumidity"` // %
			WindSpeed        measurement `json:"windSpeed"`        // km/h
		} `json:"properties"`
	}
	if err := getJSON(ctx, station.ID+"/observations/latest", &obs); err != nil {
		return nil, err
	}
	o := obs.Properties
	return &currentWeather{
		Station:      station.Properties.Name,
		ObservedAt:   o.Timestamp,
		Description:  o.TextDescription,
		TemperatureF: convert(o.Temperature.Value, func(c float64) float64 { return c*9/5 + 32 }),
		HumidityPct:  convert(o.RelativeHumidity.Value, func(h float64) float64 { return h }),
		WindMPH:      convert(o.WindSpeed.Value, func(kmh float64) float64 { return kmh / 1.609344 }),
	}, nil
}

// convert applies f to a measurement that may be missing (NWS uses null),
// rounding to one decimal.
func convert(v *float64, f func(float64) float64) *float64 {
	if v == nil {
		return nil
	}
	out := math.Round(f(*v)*10) / 10
	return &out
}

func formatCoord(v float64) string {
	return strconv.FormatFloat(math.Round(v*1e4)/1e4, 'f', -1, 64)
}

// getJSON GETs a URL with the identifying User-Agent and decodes the JSON
// response into out. Error responses include NWS's "detail" message if any.
func getJSON(ctx context.Context, rawURL string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", WeatherUserAgent)
	req.Header.Set("Accept", "application/geo+json, application/json")

	resp, err := weatherHTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var problem struct {
			Detail string `json:"detail"`
		}
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		json.Unmarshal(body, &problem)
		if problem.Detail != "" {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, problem.Detail)
		}
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
