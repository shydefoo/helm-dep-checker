package checker

import (
	"errors"
	"fmt"
	"log"

	"github.com/golang-collections/collections/queue"
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

func getDepHash(c *chart.Chart, d *chart.Dependency) string {
	if d.Name == JobChartName {
		job, err := GetJobInfoForDep(c, d)
		if err != nil {
			panic(err)
		}
		return fmt.Sprintf("%s-%s-%s", job.Chart, job.Version, job.Release)
	}
	return fmt.Sprintf("%s-%s", d.Name, d.Version)
}

func GetJobInfoForDep(ownerChart *chart.Chart, d *chart.Dependency) (*JobValues, error) {
	vals := ownerChart.Values
	nameToUse := nameOrAlias(d)
	var jv *JobValues
	var hc map[string]interface{}
	if values, ok := vals[nameToUse]; ok {
		jv = &JobValues{}
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
	if jv == nil {
		return nil, errors.New("Job is nil")
	}
	return jv, nil
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
	var jv *JobValues
	var hc map[string]interface{}
	if values, ok := parent.Values[name]; ok {
		jv = &JobValues{}
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
	return jv
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

func buildStore(commonDepList []*ChartW) {}

func WalkGraph(g *Graph) map[string]*Report {
	log.Println("walkgraph")
	report := make(map[string]*Report)

	var traverse func(c *ChartW) []*ChartW
	traverse = func(c *ChartW) []*ChartW {
		log.Println("traversing", c.Name())
		currentChash := c.ChartHash

		// Stores existing dependencies and new dependencies from grandchild charts
		depList := []*ChartW{}
		commonDep := []*ChartW{}
		q := queue.New()

		newDepsFound := false
		// stores mapping between child chart and list of ancestor charts
		depMap := make(map[string][]*ChartW)
		existingDeps := g.GMap[currentChash]
		for _, d := range existingDeps {
			q.Enqueue(d)
		}
		if len(existingDeps) == 0 {
			log.Println("done traversing", c.Name(), "no children")
			return depList
		}
		depSet := make(map[string]*ChartW)
		for q.Len() > 0 {
			d := q.Dequeue().(*ChartW)
			if _, ok := depSet[d.ChartHash]; !ok {
				depSet[d.ChartHash] = d
			}
			grandChildDeps := traverse(d)
			for _, dGrand := range grandChildDeps {
				if _, ok := depMap[dGrand.ChartHash]; !ok {
					depMap[dGrand.ChartHash] = []*ChartW{d}
				} else {
					depMap[dGrand.ChartHash] = append(depMap[dGrand.ChartHash], d)
					commonDep = append(commonDep, dGrand)
					if _, ok := depSet[dGrand.ChartHash]; !ok {
						depSet[dGrand.ChartHash] = dGrand
						log.Printf("New Deps found: %s", dGrand.ChartHash)
						newDepsFound = true
						q.Enqueue(dGrand)
					}
				}
			}
		}
		// check if existing deps contains new dep, else add to dep Set
		// for _, d := range commonDep {
		// 	if _, ok := depSet[d.ChartHash]; !ok {
		// 		depSet[d.ChartHash] = d
		// 		log.Printf("New Deps found: %s", d.ChartHash)
		// 		newDepsFound = true
		// 		newDeps = append(newDeps, d)
		// 	}
		// }
		for _, v := range depSet {
			depList = append(depList, v)
		}
		// Update chart's dependencies in graph
		g.GMap[currentChash] = depList
		if newDepsFound {
			report[currentChash] = &Report{FullDeps: depList, LookUp: depMap}
		}
		log.Println("done traversing", c.Name())
		return depList
	}

	for k, v := range g.RMap {
		if v {
			log.Println("traversing", k)
			_ = traverse(g.CMap[k])
		}
	}

	return report
}
