package main

import (
	"context"
	"fmt"
	_ "image/png"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	// REST routing
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	// Logging
	log "github.com/sirupsen/logrus"

	//Configuration
	"github.com/pborman/getopt/v2"
	"github.com/spf13/viper"

	// Prometheus metrics
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// programName is the name string we use
const programName string = "timeseries_mbgl_maps"

// programVersion is the version string we use
const programVersion string = "0.1"

// var programVersion string

func init() {
	viper.SetDefault("HttpHost", "0.0.0.0")
	viper.SetDefault("HttpPort", 5800)
	viper.SetDefault("MbglUrl", "https://eahazardswatch.icpac.net/mbgl-renderer/render")
	viper.SetDefault("BasePath", "/")
	viper.SetDefault("fontFilePath", "./fonts/OpenSans-Bold.ttf")
	viper.SetDefault("ImageWidth", 100)
	viper.SetDefault("ImageHeight", 100)
	viper.SetDefault("TextHeight", 40)
	viper.SetDefault("LeftLabelsWidth", 70)
	viper.SetDefault("RightPadding", 20)
	viper.SetDefault("ImagePadding", 1)
	viper.SetDefault("EnableMetrics", false) // Prometheus metrics
	viper.SetDefault("CORSOrigins", []string{"*"})
	viper.SetDefault("Timeout", 30)
	viper.SetDefault("Debug", false)
}

func main() {

	// Read the commandline
	flagDebugOn := getopt.BoolLong("debug", 'd', "log debugging information")
	flagConfigFile := getopt.StringLong("config", 'c', "", "full path to config file", "config.toml")
	flagHelpOn := getopt.BoolLong("help", 'h', "display help output")
	flagVersionOn := getopt.BoolLong("version", 'v', "display version number")
	getopt.Parse()

	if *flagHelpOn {
		getopt.PrintUsage(os.Stdout)
		os.Exit(1)
	}

	if *flagVersionOn {
		fmt.Printf("%s %s\n", programName, programVersion)
		os.Exit(0)
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("tsm")

	// Commandline over-rides config file for debugging
	if *flagDebugOn {
		viper.Set("Debug", true)
		log.SetLevel(log.TraceLevel)
	}

	if *flagConfigFile != "" {
		viper.SetConfigFile(*flagConfigFile)
	} else {
		viper.SetConfigName(programName)
		viper.SetConfigType("toml")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("/config")
	}

	// Report our status
	log.Infof("%s %s", programName, programVersion)
	log.Info("Run with --help parameter for commandline options")

	// Read environment configuration first
	if mbglURL := os.Getenv("MBGL_URL"); mbglURL != "" {
		viper.Set("MbglUrl", mbglURL)
		log.Info("Using mbgl render url from environment variable MBGL_URL")
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Debugf("viper.ConfigFileNotFoundError: %s", err)
		} else {
			if _, ok := err.(viper.UnsupportedConfigError); ok {
				log.Debugf("viper.UnsupportedConfigError: %s", err)
			} else {
				log.Fatalf("Configuration file error: %s", err)
			}
		}
	} else {
		if cf := viper.ConfigFileUsed(); cf != "" {
			log.Infof("Using config file: %s", cf)
		} else {
			log.Info("Config file: none found, using defaults")
		}
	}

	basePath := viper.GetString("BasePath")
	log.Infof("Serving HTTP  at %s/", formatBaseURL(fmt.Sprintf("http://%s:%d",
		viper.GetString("HttpHost"), viper.GetInt("HttpPort")), basePath))

	// Get to work
	handleRequests()
}

/******************************************************************************/
func requestRenderMapsGrid(w http.ResponseWriter, r *http.Request) error {
	log.WithFields(log.Fields{
		"event": "request",
		"topic": "rendermapsgrid",
	}).Trace("requestRenderMapsGrid")

	dc, err := generateMapsGrid(w, r)
	if err != nil {
		return err
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "image/png")
	dc.EncodePNG(w)

	return nil
}

/******************************************************************************/

// tsAppError is an optional error structure functions can return
// if they want to specify the particular HTTP error code to be used
// in their error return
type tsAppError struct {
	HTTPCode int
	SrcErr   error
	Topic    string
	Message  string
}

// Error prints out a reasonable string format
func (tsae tsAppError) Error() string {
	if tsae.Message != "" {
		return fmt.Sprintf("%s\n%s", tsae.Message, tsae.SrcErr.Error())
	}
	return tsae.SrcErr.Error()
}

// tsMapsHandler is a function handler that can replace the
// existing handler and provide richer error handling, see below and
// https://blog.golang.org/error-handling-and-go
type tsMapsHandler func(w http.ResponseWriter, r *http.Request) error

// ServeHTTP logs as much useful information as possible in
// a field format for potential Json logging streams
// as well as returning HTTP error response codes on failure
// so clients can see what is going on
func (fn tsMapsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.WithFields(log.Fields{
		"method": r.Method,
		"url":    r.URL,
	}).Infof("%s %s", r.Method, r.URL)

	if err := fn(w, r); err != nil {
		if hdr, ok := r.Header["x-correlation-id"]; ok {
			log.WithField("correlation-id", hdr[0])
		}
		if e, ok := err.(tsAppError); ok {
			if e.HTTPCode == 0 {
				e.HTTPCode = 500
			}
			if e.Topic != "" {
				log.WithField("topic", e.Topic)
			}
			log.WithField("key", e.Message)
			log.WithField("src", e.SrcErr.Error())
			log.Error(err)
			http.Error(w, e.Error(), e.HTTPCode)
		} else {
			log.Error(err)
			http.Error(w, err.Error(), 500)
		}
	}
}

/******************************************************************************/

func tsMapsRouter() *mux.Router {
	// creates a new instance of a mux router
	r := mux.NewRouter().
		StrictSlash(true).
		PathPrefix(
			"/" +
				strings.TrimLeft(viper.GetString("BasePath"), "/"),
		).
		Subrouter()
	// rendering page
	r.Handle("/", tsMapsHandler(requestRenderMapsGrid)).Methods("POST")

	if viper.GetBool("EnableMetrics") {
		r.Handle("/metrics", promhttp.Handler())
	}
	return r
}

func handleRequests() {
	// Get a configured router
	r := tsMapsRouter()

	// Allow CORS from anywhere
	corsOrigins := viper.GetStringSlice("CORSOrigins")
	corsOpt := handlers.AllowedOrigins(corsOrigins)

	// Set a writeTimeout for the http server.
	// This value is the application's DbTimeout config setting plus a
	// grace period. The additional time allows the application to gracefully
	// handle timeouts on its own, canceling outstanding database queries and
	// returning an error to the client, while keeping the http.Server
	// WriteTimeout as a fallback.
	writeTimeout := (time.Duration(viper.GetInt("Timeout") + 5)) * time.Second

	// more "production friendly" timeouts
	// https://blog.simon-frey.eu/go-as-in-golang-standard-net-http-config-will-break-your-production/#You_should_at_least_do_this_The_easy_path
	s := &http.Server{
		ReadTimeout:  1 * time.Second,
		WriteTimeout: writeTimeout,
		Addr:         fmt.Sprintf("%s:%d", viper.GetString("HttpHost"), viper.GetInt("HttpPort")),
		Handler:      handlers.CompressHandler(handlers.CORS(corsOpt)(r)),
	}

	// start http service
	go func() {
		// ListenAndServe returns http.ErrServerClosed when the server receives
		// a call to Shutdown(). Other errors are unexpected.
		if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// wait here for interrupt signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	<-sig

	// Interrupt signal received:  Start shutting down
	log.Infoln("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	s.Shutdown(ctx)

	log.Infoln("Server stopped.")
}

/******************************************************************************/
