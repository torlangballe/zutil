package zweather

import (
	"fmt"
	"time"

	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zhttp"
)

// https://api.met.no/weatherapi/locationforecast/2.0/documentation

type MetNoWeather struct{}

type Measurements struct {
	AirPressureAtSeaLevel float64 `json:"air_pressure_at_sea_level"`
	AirTemperature        float64 `json:"air_temperature"`
	CloudAreaFraction     float64 `json:"cloud_area_fraction"`
	PrecipitationAmount   float64 `json:"precipitation_amount"`
	RelativeHumidity      float64 `json:"relative_humidity"`
	WindFromDirection     float64 `json:"wind_from_direction"`
	WindSpeed             float64 `json:"wind_speed"`
}

type Units struct {
	AirPressureAtSeaLevel string `json:"air_pressure_at_sea_level"`
	AirTemperature        string `json:"air_temperature"`
	CloudAreaFraction     string `json:"cloud_area_fraction"`
	PrecipitationAmount   string `json:"precipitation_amount"`
	RelativeHumidity      string `json:"relative_humidity"`
	WindFromDirection     string `json:"wind_from_direction"`
	WindSpeed             string `json:"wind_speed"`
}

type Period struct {
	Summary struct {
		SymbolCode string `json:"symbol_code"`
	} `json:"summary"`
	Details struct {
		PrecipitationAmount float64 `json:"precipitation_amount"`
	} `json:"details"`
}

type LocationForcast struct {
	Type     string `json:"type"`
	Geometry struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`
	Properties struct {
		Meta struct {
			UpdatedAt time.Time `json:"updated_at"`
			Units     Units     `json:"units"`
		} `json:"meta"`
		Timeseries []struct {
			Time time.Time `json:"time"`
			Data struct {
				Instant struct {
					Details Measurements
				} `json:"instant"`
				Next1Hours  Period `json:"next_1_hours"`
				Next6Hours  Period `json:"next_6_hours"`
				Next12Hours Period `json:"next_12_hours"`
			} `json:"data,omitempty"`
		} `json:"timeseries"`
	} `json:"properties"`
}

func GetLocalForcast(geoPos zgeo.Pos) ([]Forcast, error) {
	var loc LocationForcast
	var forcasts []Forcast
	surl := "https://api.met.no/weatherapi/locationforecast/2.0/compact"
	args := map[string]string{
		"lat": fmt.Sprint(geoPos.Y),
		"lon": fmt.Sprint(geoPos.X),
	}
	surl, _ = zhttp.MakeURLWithArgs(surl, args)
	params := zhttp.MakeParameters()
	params.Headers["User-Agent"] = "UserAgent-etheros.online"
	_, err := zhttp.Get(surl, params, &loc)
	if err != nil {
		return nil, err
	}
	for _, t := range loc.Properties.Timeseries {
		var f Forcast
		f.Time = t.Time
		f.Duration = time.Hour
		f.AirPressureAtSeaLevelHPA = t.Data.Instant.Details.AirPressureAtSeaLevel
		f.AirTemperatureCelcius = t.Data.Instant.Details.AirTemperature
		f.CloudAreaFractionPercent = t.Data.Instant.Details.CloudAreaFraction
		f.PrecipitationAmountMM = t.Data.Instant.Details.PrecipitationAmount

		f.RelativeHumidityPercent = t.Data.Instant.Details.RelativeHumidity
		f.WindFromDirectionDegrees = t.Data.Instant.Details.WindFromDirection
		f.WindSpeedMPS = t.Data.Instant.Details.WindSpeed
		f.SymbolCode = t.Data.Next1Hours.Summary.SymbolCode
		forcasts = append(forcasts, f)
	}
	return forcasts, nil
}
