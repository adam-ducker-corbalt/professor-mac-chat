package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// fakeWeatherServices stands in for Nominatim and the NWS, returning a forecast
// for Joplin, MO. It records each path requested and its User-Agent.
func fakeWeatherServices(t *testing.T) (requests *[]string) {
	t.Helper()
	requests = &[]string{}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requests = append(*requests, r.URL.Path+" UA="+r.UserAgent())
		switch {
		case r.URL.Path == "/search":
			if r.URL.Query().Get("q") == "Atlantis" {
				w.Write([]byte(`[]`))
				return
			}
			w.Write([]byte(`[{"lat":"37.08423","lon":"-94.51328","display_name":"Joplin, Jasper County, Missouri, United States"}]`))
		case r.URL.Path == "/points/37.0842,-94.5133":
			w.Write([]byte(`{"properties":{
				"forecast":"` + srv.URL + `/gridpoints/SGF/30,40/forecast",
				"observationStations":"` + srv.URL + `/gridpoints/SGF/30,40/stations",
				"timeZone":"America/Chicago",
				"relativeLocation":{"properties":{"city":"Joplin","state":"MO"}}}}`))
		case r.URL.Path == "/gridpoints/SGF/30,40/forecast":
			w.Write([]byte(`{"properties":{"periods":[
				{"name":"Today","startTime":"2026-10-27T09:00:00-05:00","temperature":68,"temperatureUnit":"F","windSpeed":"10 mph","windDirection":"S","shortForecast":"Sunny","detailedForecast":"Sunny, with a high near 68.","probabilityOfPrecipitation":{"value":null}},
				{"name":"Tonight","startTime":"2026-10-27T18:00:00-05:00","temperature":49,"temperatureUnit":"F","windSpeed":"5 mph","windDirection":"S","shortForecast":"Clear","detailedForecast":"Clear, with a low around 49.","probabilityOfPrecipitation":{"value":0}},
				{"name":"Wednesday","startTime":"","temperature":72,"temperatureUnit":"F","windSpeed":"","windDirection":"","shortForecast":"","detailedForecast":"","probabilityOfPrecipitation":{"value":20}},
				{"name":"Wednesday Night","startTime":"","temperature":55,"temperatureUnit":"F","windSpeed":"","windDirection":"","shortForecast":"","detailedForecast":"","probabilityOfPrecipitation":{"value":40}},
				{"name":"Thursday","startTime":"","temperature":60,"temperatureUnit":"F","windSpeed":"","windDirection":"","shortForecast":"","detailedForecast":"","probabilityOfPrecipitation":{"value":60}}
			]}}`))
		case r.URL.Path == "/gridpoints/SGF/30,40/stations":
			w.Write([]byte(`{"features":[{"id":"` + srv.URL + `/stations/KJLN","properties":{"name":"Joplin Regional Airport"}}]}`))
		case r.URL.Path == "/stations/KJLN/observations/latest":
			w.Write([]byte(`{"properties":{"timestamp":"2026-10-27T13:53:00+00:00","textDescription":"Clear",
				"temperature":{"value":20},"relativeHumidity":{"value":45.26},"windSpeed":{"value":16.668}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"detail":"Data Unavailable For Requested Point"}`))
		}
	}))
	t.Cleanup(srv.Close)

	oldGeocode, oldAPI, oldAgent := geocodeURL, weatherAPIURL, WeatherUserAgent
	geocodeURL, weatherAPIURL, WeatherUserAgent = srv.URL+"/search", srv.URL, "test-agent"
	t.Cleanup(func() { geocodeURL, weatherAPIURL, WeatherUserAgent = oldGeocode, oldAPI, oldAgent })
	return requests
}

func TestWeatherForecast(t *testing.T) {
	requests := fakeWeatherServices(t)

	report, err := getWeatherForecast(context.Background(), "Joplin, MO")
	if err != nil {
		t.Fatal(err)
	}
	if report.NWSLocation != "Joplin, MO" || report.TimeZone != "America/Chicago" {
		t.Errorf("location = %q, time zone = %q", report.NWSLocation, report.TimeZone)
	}
	if len(report.Forecast) != forecastPeriods {
		t.Fatalf("got %d forecast periods, want %d", len(report.Forecast), forecastPeriods)
	}
	if today := report.Forecast[0]; today.Name != "Today" || today.Temperature != 68 || today.Wind != "10 mph S" {
		t.Errorf("first period = %+v", today)
	}
	if c := report.Current; c == nil || *c.TemperatureF != 68 || *c.WindMPH != 10.4 || c.Station != "Joplin Regional Airport" {
		t.Errorf("current conditions = %+v", c)
	}

	// Coordinates are trimmed to NWS's 4 decimal places, and every request
	// identifies the app.
	for _, r := range *requests {
		if !strings.HasSuffix(r, "UA=test-agent") {
			t.Errorf("request %s didn't send the User-Agent", r)
		}
	}
}

func TestWeatherUnknownPlace(t *testing.T) {
	fakeWeatherServices(t)
	got := runTool(context.Background(), "get_weather_forecast", `{"location":"Atlantis"}`)
	if !strings.Contains(got, "couldn't find a US location") {
		t.Errorf("got %s", got)
	}
}

// Calls the real Nominatim and NWS APIs. Skipped unless WEATHER_LIVE=1:
//
//	WEATHER_LIVE=1 go test ./internal/app/chat -run Live -v
func TestWeatherLive(t *testing.T) {
	if os.Getenv("WEATHER_LIVE") != "1" {
		t.Skip("set WEATHER_LIVE=1 to call the real weather APIs")
	}
	for _, location := range []string{"Joplin, MO", "90210"} {
		report, err := getWeatherForecast(context.Background(), location)
		if err != nil {
			t.Fatalf("%s: %v", location, err)
		}
		out, _ := json.MarshalIndent(report, "", "  ")
		t.Logf("%s:\n%s", location, out)
	}
}
