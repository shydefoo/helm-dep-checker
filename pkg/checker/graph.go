package checker

import (
	"fmt"
	"log"

	"helm.sh/helm/v3/pkg/chart"
)

type graphM map[string][]*chart.Chart
type chartM map[string]*chart.Chart

type Graph struct {
	GMap graphM
	CMap chartM
}

const (
	JobChartName = "generic-helm-installer"
)

func getChartHash(c *chart.Chart) string {
	if c.Name() == JobChartName {
		return getGenInstallerHash(c)
	}
	return fmt.Sprintf("%s-%s", c.Name(), c.Metadata.Version)
}

func getGenInstallerHash(c *chart.Chart) string {
	dep := GetDependencyObj(c)
	return fmt.Sprintf("%s-%s", dep.Name, dep.Version)
}

func getDepHash(d *chart.Dependency) string {
	return fmt.Sprintf("%s-%s", d.Name, d.Version)
}

func ConstructGraph(charts []*chart.Chart) (*Graph, error) {
	g := make(map[string][]*chart.Chart)
	cM := make(map[string]*chart.Chart)
	var _makeGraph func(*chart.Chart)
	_makeGraph = func(c *chart.Chart) {
		chash := getChartHash(c)
		if _, ok := cM[chash]; !ok {
			cM[chash] = c
		}
		if _, ok := g[chash]; !ok {
			g[chash] = c.Dependencies()
			for _, d := range c.Dependencies() {
				_makeGraph(d)
			}
		}
	}
	for _, c := range charts {
		if c.IsRoot() {
			// Only construct graph for root charts, dependency charts will get added in through MakeGraph
			_makeGraph(c)
		}
	}
	graph := &Graph{
		GMap: g, CMap: cM,
	}
	return graph, nil
}

type Report struct {
	FullDeps []*chart.Chart
	LookUp   map[string][]*chart.Chart
}

func WalkGraph(g *Graph, rootChart *chart.Chart) map[string]*Report {
	log.Println("walkgraph")
	report := make(map[string]*Report)
	var traverse func(c *chart.Chart) []*chart.Chart
	traverse = func(c *chart.Chart) []*chart.Chart {
		currentChash := getChartHash(c)
		depList := []*chart.Chart{}
		commonDep := []*chart.Chart{}

		// stores mapping between child chart and list of parent charts
		depMap := make(map[string][]*chart.Chart)
		existingDeps := g.GMap[getChartHash(c)]
		if len(existingDeps) == 0 {
			return depList
		}
		for _, d := range existingDeps {
			childDeps := traverse(d)
			for _, dGrand := range childDeps {
				if _, ok := depMap[getChartHash(dGrand)]; !ok {
					depMap[getChartHash(dGrand)] = []*chart.Chart{d}
				} else {
					depMap[getChartHash(dGrand)] = append(depMap[getChartHash(dGrand)], d)
					commonDep = append(commonDep, dGrand)
				}
			}
		}
		depSet := make(map[string]*chart.Chart)
		for _, s := range existingDeps {
			chash := getChartHash(s)
			if _, ok := depSet[chash]; !ok {
				depSet[chash] = s
			}
		}
		newDepsFound := false
		for _, d := range commonDep {
			chash := getChartHash(d)
			if _, ok := depSet[chash]; !ok {
				depSet[chash] = d
				newDepsFound = true
			}
		}
		for _, v := range depSet {
			depList = append(depList, v)
		}
		// Update chart's dependencies in graph
		g.GMap[currentChash] = depList
		if newDepsFound {
			report[currentChash] = &Report{FullDeps: depList, LookUp: depMap}
		}
		return depList
	}
	_ = traverse(rootChart)
	return report
}
