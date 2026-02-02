package ingest

type Analysis struct {
	SchemaVersion string   `json:"schema_version"`
	Date          string   `json:"date"`
	SiteID        string   `json:"site_id"`
	DeviceID      string   `json:"device_id"`
	Site          *SiteRef `json:"site"`
	Sensors       []Sensor `json:"sensors"`
}

type SiteRef struct {
	ID string `json:"id"`
}

type Sensor struct {
	SensorDir  string                 `json:"sensor_dir"`
	SensorType string                 `json:"sensor_type"`
	Metrics    map[string]interface{} `json:"metrics"`
	Frames     map[string]interface{} `json:"frames"`
	Issues     []Issue                `json:"issues"`
}

type Issue struct {
	Type     string      `json:"type"`
	Count    interface{} `json:"count"`
	Examples interface{} `json:"examples"`
}
