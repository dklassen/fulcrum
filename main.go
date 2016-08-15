package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
)

var (
	//	re_inside_whtsp = regexp.MustCompile(`[\s\p{Zs}]{2,}`)
	token           = flag.String("token", "REQUIRED", "Lever api token")
	debug           = flag.Bool("debug", false, "Enable debug logging")
	download        = flag.Bool("download", true, "Flag to switch upload/download")
	input           = flag.String("input", "", "File to input and update Lever with")
	endpoint        = flag.String("endpoint", "", "Lever endpoint to hit")
	createdAtStart  = flag.String("createdAtStart", "", "Set createdAtStart field")
	archivedAtStart = flag.String("archivedAtStart", "", "Set archivedAtStart field")
	performAs       = flag.String("performAs", "", "Set perform_as query parameter")
)

type Config struct {
	LeverToken      string
	Debug           bool
	Download        bool
	Input           string
	Endpoint        string
	CreatedAtStart  string
	ArchivedAtStart string
	PerformAs       string
}

func LoadFromFlags() (*Config, error) {
	flag.Parse()

	return &Config{
		LeverToken:      *token,
		Debug:           *debug,
		Input:           *input,
		Endpoint:        *endpoint,
		CreatedAtStart:  *createdAtStart,
		ArchivedAtStart: *archivedAtStart,
		PerformAs:       *performAs,
	}, nil
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {
	if len(os.Args) == 1 {
		flag.Usage()
	}

	config, _ := LoadFromFlags()
	apiToken = config.LeverToken
	if apiToken == "" {
		logrus.Fatal("No api token given use --token= to specify one.")
	}

	queryParams := []QueryParam{}
	if config.CreatedAtStart != "" {
		queryParams = append(queryParams, QueryParam{Field: "created_at_start", Value: config.CreatedAtStart})
	}

	if config.ArchivedAtStart != "" {
		queryParams = append(queryParams, QueryParam{Field: "archived_at_start", Value: config.ArchivedAtStart})
	}

	if config.PerformAs != "" {
		queryParams = append(queryParams, QueryParam{Field: "perform_as", Value: config.PerformAs})
	}

	endpoint, ok := registeredEndpoints[config.Endpoint]
	if !ok {
		logrus.Fatal("Looks like the endpoint is not registered")
	}
	endpoint.QueryParams = queryParams

	handler := endpoint.Handler
	state := NewCheckpoint(endpoint.Type)
	err := handler(endpoint, config.Input, state)
	if err != nil {
		logrus.Fatal(err)
	}
	logrus.Info("All done")
}
