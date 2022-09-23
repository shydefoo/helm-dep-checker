package main

import (
	"flag"
	"log"

	"os"

	"caraml-dev/caraml-dep-checker/pkg/checker"
)

var chartPath = flag.String("p", "", "Path the helm charts")
var rCName = flag.String("r", "root", "Root chart name")
var rCVersion = flag.String("v", "0.1.0", "Root chart version")

func main() {
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
