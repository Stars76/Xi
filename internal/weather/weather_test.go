package weather

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMapWMOCode(t *testing.T) {
	cases := map[int]string{
		0:  "clear",
		3:  "clear",
		45: "fog",
		48: "fog",
		51: "rain",
		63: "rain",
		80: "rain",
		71: "snow",
		86: "snow",
		95: "storm",
		99: "storm",
	}
	for code, want := range cases {
		if got := MapWMOCode(code); got != want {
			t.Fatalf("MapWMOCode(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestGeocodeAndFetchCurrent(t *testing.T) {
	geoSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("name") != "杭州" {
			t.Errorf("unexpected city query: %s", r.URL.Query().Get("name"))
		}
		_, _ = w.Write([]byte(`{"results":[{"name":"杭州","admin1":"浙江","country":"中国","latitude":30.2741,"longitude":120.1551}]}`))
	}))
	defer geoSrv.Close()

	fcSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"current":{"weather_code":63,"temperature_2m":18.4}}`))
	}))
	defer fcSrv.Close()

	cl := &Client{
		GeoBaseURL:      geoSrv.URL,
		ForecastBaseURL: fcSrv.URL,
		HTTPClient:      geoSrv.Client(),
	}

	geo, err := cl.Geocode(context.Background(), "杭州")
	if err != nil {
		t.Fatal(err)
	}
	if geo.Name != "杭州 · 浙江" || geo.Lat < 30 || geo.Lon < 120 {
		t.Fatalf("unexpected geo result: %+v", geo)
	}

	cur, err := cl.FetchCurrent(context.Background(), geo.Lat, geo.Lon)
	if err != nil {
		t.Fatal(err)
	}
	if cur.Weather != "rain" || cur.TempC != 18.4 || cur.WMOCode != 63 {
		t.Fatalf("unexpected weather result: %+v", cur)
	}
}
