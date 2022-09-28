package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/shydefoo/helm-dep-checker/pkg/checker"
)

var chartPath = flag.String("p", "", "Path the helm charts")
var debug = flag.Bool("debug", false, "sets log level to debug")

func main() {
	// UNIX Time is faster and smaller than most timestamps
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.With().Caller().Logger()

	flag.Parse()

	// Default level for this example is info, unless debug flag is present
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	flag.Parse()
	// to change the flags on the default logger
	log.Print("chartPath", *chartPath)
	mc := checker.Checker{ChartDir: *chartPath}
	c, err := checker.GetCharts(mc.ChartDir)
	if err != nil {
		panic(err)
	}
	g, err := checker.ConstructGraph(c)
	if err != nil {
		panic(err)
	}
	report := checker.WalkGraph(g)
	changed, changes, err := mc.CollectChanges(report, g)
	if err != nil {
		panic(err)
	}
	if changed {
		log.Print("Changes required:")
		fmt.Println(changes)
		os.Exit(1)
	} else {
		log.Info().Msg("SUCCESS, NO CHANGES REQUIRED")
	}

}
