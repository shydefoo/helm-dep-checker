package main

import (
	"flag"
	"log"

	"os"

	"caraml-dev/caraml-dep-checker/pkg/checker"
)

var chartPath = flag.String("p", "", "Path the helm charts")

func main() {
	flag.Parse()
	log.Println("chartPath", *chartPath)
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
		log.Println("Changes required:")
		log.Fatal(changes)
		os.Exit(1)
	}

}
