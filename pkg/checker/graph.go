package checker

import (
	"errors"
	"fmt"
	"log"

	"helm.sh/helm/v3/pkg/chart"
)

type graphM map[string][]*ChartW
type chartM map[string]*ChartW

type Graph struct {
	GMap graphM
	CMap chartM
	RMap map[string]bool
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
		jHash, _ := getGenInstallerHash(c)
		return jHash
	}
	return getNormalHash(c)
}

func getNormalHash(c *chart.Chart) string {
	return fmt.Sprintf("%s-%s", c.Name(), c.Metadata.Version)
}

func getGenInstallerHash(c *chart.Chart) (string, *JobValues) {
	job := GetJobInfo(c)
	if job == nil {
		return "", job
	}
	return fmt.Sprintf("%s-%s-%s", job.Chart, job.Version, job.Release), job
}

func getGenInstallerHashStrict(c *chart.Chart) (string, *JobValues, error) {
	job := GetJobInfo(c)
	if job == nil {
		return "", job, errors.New("Job is nil")
	}
	return fmt.Sprintf("%s-%s-%s", job.Chart, job.Version, job.Release), job, nil
}
func getDepHash(d *chart.Dependency) string {
	return fmt.Sprintf("%s-%s", d.Name, d.Version)
}

func getDepHashFromParent(pC *ChartW, d *chart.Dependency) string {
	if d.Name == JobChartName {
		// jHash, _ := getGenInstallerHash(pC)
		return pC.ChartHash
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

func ConstructGraph(charts []*ChartW) (*Graph, error) {
	g := make(map[string][]*ChartW)
	cM := make(map[string]*ChartW)
	rM := make(map[string]bool)
	var _makeGraph func(*ChartW)
	_makeGraph = func(c *ChartW) {
		chash := c.ChartHash
		if _, ok := cM[chash]; !ok {
			cM[chash] = c
		}
		if _, ok := g[chash]; !ok {
			g[chash] = c.DepsW
			for _, d := range c.DepsW {
				rM[d.ChartHash] = false
				_makeGraph(d)
			}
		}
	}
	for _, c := range charts {
		// set all charts to be roots
		rM[c.ChartHash] = true
	}
	for _, c := range charts {
		// Only construct graph for root charts, dependency charts will get added in through MakeGraph
		_makeGraph(c)
	}
	graph := &Graph{
		GMap: g, CMap: cM, RMap: rM,
	}
	return graph, nil
}

type Report struct {
	FullDeps []*ChartW
	LookUp   map[string][]*ChartW
}

func WalkGraph(g *Graph) map[string]*Report {
	log.Println("walkgraph")
	report := make(map[string]*Report)
	var traverse func(c *ChartW) []*ChartW
	traverse = func(c *ChartW) []*ChartW {
		currentChash := c.ChartHash
		depList := []*ChartW{}
		commonDep := []*ChartW{}

		// stores mapping between child chart and list of parent charts
		depMap := make(map[string][]*ChartW)
		existingDeps := g.GMap[c.ChartHash]
		if len(existingDeps) == 0 {
			return depList
		}
		for _, d := range existingDeps {
			childDeps := traverse(d)
			for _, dGrand := range childDeps {
				if _, ok := depMap[dGrand.ChartHash]; !ok {
					depMap[dGrand.ChartHash] = []*ChartW{d}
				} else {
					depMap[dGrand.ChartHash] = append(depMap[dGrand.ChartHash], d)
					commonDep = append(commonDep, dGrand)
				}
			}
		}
		depSet := make(map[string]*ChartW)
		for _, s := range existingDeps {
			chash := s.ChartHash
			if _, ok := depSet[chash]; !ok {
				depSet[chash] = s
			}
		}
		newDepsFound := false
		for _, d := range commonDep {
			chash := d.ChartHash
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
	// _ = traverse(rootChart)
	for k, v := range g.RMap {
		if v {
			log.Println("traversing", k)
			_ = traverse(g.CMap[k])
		}
	}

	return report
}
