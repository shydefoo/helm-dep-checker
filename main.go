package main

import (
	"fmt"
	"log"

	"os"

	"caraml-dev/caraml-dep-checker/pkg/checker"
)

func main() {
	mc := checker.Checker{ChartDir: "/tmp/test-charts/test_chart_3478466454", RootChartName: "root", RootChartVer: "0.1.0"}
	c, err := checker.GetCharts(mc.ChartDir)
	if err != nil {
		panic(err)
	}
	g, err := checker.ConstructGraph(c)
	if err != nil {
		panic(err)
	}
	report := checker.WalkGraph(g, g.CMap[fmt.Sprintf("%s-%s", mc.RootChartName, mc.RootChartVer)])
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
