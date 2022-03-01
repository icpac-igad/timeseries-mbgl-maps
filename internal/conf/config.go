package conf

import (
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Configuration for system
var Configuration Config

func setDefaultConfig() {
	viper.SetDefault("Server.HttpHost", "0.0.0.0")
	viper.SetDefault("Server.HttpPort", 9000)
	viper.SetDefault("Server.UrlBase", "")
	viper.SetDefault("Server.BasePath", "")
	viper.SetDefault("Server.CORSOrigins", "*")
	viper.SetDefault("Server.Debug", false)

	viper.SetDefault("Server.ReadTimeoutSec", 5)
	viper.SetDefault("Server.WriteTimeoutSec", 120)

	viper.SetDefault("MapsGrid.MbglUrl", "https://eahazardswatch.icpac.net/mbgl-renderer/render")
	viper.SetDefault("MapsGrid.FontFilePath", "./fonts/OpenSans-Bold.ttf")
	viper.SetDefault("MapsGrid.ImageWidth", 100)
	viper.SetDefault("MapsGrid.ImageHeight", 100)
	viper.SetDefault("MapsGrid.TextHeight", 40)
	viper.SetDefault("MapsGrid.LeftLabelsWidth", 70)
	viper.SetDefault("MapsGrid.RightPadding", 20)
	viper.SetDefault("MapsGrid.ImagePadding", 1)

	viper.SetDefault("Metadata.Title", "timeseries-mbgl-maps")
	viper.SetDefault("Metadata.Description", "EAHW Timeseries MBGL Maps")
}

// Config for system
type Config struct {
	Server   Server
	Metadata Metadata
	MapsGrid MapsGridConfig
}

// Server config
type Server struct {
	HttpHost        string
	HttpPort        int
	UrlBase         string
	BasePath        string
	CORSOrigins     string
	Debug           bool
	ReadTimeoutSec  int
	WriteTimeoutSec int
}

// Metadata config
type Metadata struct {
	Title       string
	Description string
}

type MbglRenderer struct {
	Url string
}

type MapsGridConfig struct {
	ImageWidth      int    `mapstructure:"ImageWidth"`
	ImageHeight     int    `mapstructure:"ImageHeight"`
	TextHeight      int    `mapstructure:"TextHeight"`
	LeftLabelsWidth int    `mapstructure:"LeftLabelsWidth"`
	RightPadding    int    `mapstructure:"RightPadding"`
	ImagePadding    int    `mapstructure:"ImagePadding"`
	FontFilePath    string `mapstructure:"FontFilePath"`
	MbglUrl         string `mapstructure:"MbglUrl"`
}

// InitConfig initializes the configuration from the config file
func InitConfig(configFilename string) {
	// --- defaults
	setDefaultConfig()

	isExplictConfigFile := configFilename != ""
	confFile := AppConfig.Name + ".toml"

	if configFilename != "" {
		viper.SetConfigFile(configFilename)
		confFile = configFilename
	} else {
		viper.SetConfigName(confFile)
		viper.SetConfigType("toml")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("/config")
		viper.AddConfigPath("/etc")
	}

	fmt.Println(configFilename)

	err := viper.ReadInConfig() // Find and read the config file

	if err != nil {
		_, isConfigFileNotFound := err.(viper.ConfigFileNotFoundError)
		errrConfRead := fmt.Errorf("fatal error reading config file: %s", err)
		isUseDefaultConfig := isConfigFileNotFound && !isExplictConfigFile
		if isUseDefaultConfig {
			confFile = "DEFAULT" // let user know config is defaulted
			log.Debug(errrConfRead)
		} else {
			log.Fatal(errrConfRead)
		}
	}

	log.Infof("Using config file: %s", viper.ConfigFileUsed())
	viper.Unmarshal(&Configuration)

	// override mbglurl with url  from env
	if mbglUrl := os.Getenv("MBGL_URL"); mbglUrl != "" {
		Configuration.MapsGrid.MbglUrl = mbglUrl
	}

	// sanitize the configuration
	Configuration.Server.BasePath = strings.TrimRight(Configuration.Server.BasePath, "/")

	//fmt.Printf("Viper: %v\n", viper.AllSettings())
	//fmt.Printf("Config: %v\n", Configuration)
}
