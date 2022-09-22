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
	JobChartName = "generic-dep-installer"
	HelmChartKey = "helmChart"
)

func IsGenericInstaller(c *chart.Chart) bool {
	return c.Name() == JobChartName
}

func getChartHash(c *chart.Chart) string {
	if IsGenericInstaller(c) {
		return getGenInstallerHash(c)
	}
	return fmt.Sprintf("%s-%s", c.Name(), c.Metadata.Version)
}

func getGenInstallerHash(c *chart.Chart) string {
	job := GetJobInfo(c)
	if job == nil {
		return fmt.Sprintf("INSTALLER_HASH_NOT_FOUND %s", c.Name())
	}
	return fmt.Sprintf("%s-%s-%s", job.Chart, job.Version, job.Release)
}

func getDepHash(d *chart.Dependency) string {
	return fmt.Sprintf("%s-%s", d.Name, d.Version)
}

func getDepHashFromParent(pC *chart.Chart, d *chart.Dependency) string {
	if d.Name == JobChartName {
		return getGenInstallerHash(pC)
	}
	return fmt.Sprintf("%s-%s", d.Name, d.Version)
}

// GetJobInfo returns matching dependency chart from parent charts Metadata.Dependency
// field. Only applies to immediate parent of chart
func GetJobInfo(c *chart.Chart) *JobValues {
	parent := c.Parent()
	if parent == nil {
		return nil
	}
	var gI *chart.Dependency
	gI = nil
	for _, d := range parent.Metadata.Dependencies {
		if d.Name == c.Name() {
			gI = d
			break
		}
	}
	if gI == nil {
		log.Println("Cannot find job Values for chart ", c.Name())
		return nil
	}
	name := nameOrAlias(gI)
	jv := JobValues{}
	var hc map[string]interface{}
	if values, ok := parent.Values[name]; ok {
		val := values.(map[string]interface{})
		hc, _ = val["helmChart"].(map[string]interface{})
		if v, ok := hc["repository"]; ok {
			jv.Repository = v.(string)
		}
		if v, ok := hc["chart"]; ok {
			jv.Chart = v.(string)
		}
		if v, ok := hc["version"]; ok {
			jv.Version = v.(string)
		}
		if v, ok := hc["namespace"]; ok {
			jv.Namespace = v.(string)
		}
		if v, ok := hc["release"]; ok {
			jv.Release = v.(string)
		}
	}
	return &jv
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
				log.Printf("New Deps found: %s", chash)
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
