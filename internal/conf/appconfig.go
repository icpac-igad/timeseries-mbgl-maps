package conf

var setVersion string = "1.0"

// AppConfiguration is the set of global application configuration constants.
type AppConfiguration struct {
	// AppName name of the software
	Name string
	// AppVersion version number of the software
	Version  string
	EnvDBURL string
}

var AppConfig = AppConfiguration{
	Name:    "timeseries_mbgl_maps",
	Version: setVersion,
}
