package zweather

import (
	"time"
)

type Forcast struct {
	Time                     time.Time
	Duration                 time.Duration
	AirPressureAtSeaLevelHPA float64 `json:"air_pressure_at_sea_level"`
	AirTemperatureCelcius    float64 `json:"air_temperature"`
	CloudAreaFractionPercent float64 `json:"cloud_area_fraction"`
	PrecipitationAmountMM    float64 `json:"precipitation_amount"`
	RelativeHumidityPercent  float64 `json:"relative_humidity"`
	WindFromDirectionDegrees float64 `json:"wind_from_direction"`
	WindSpeedMPS             float64 `json:"wind_speed"`
}
