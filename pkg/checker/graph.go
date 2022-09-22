package checker

import (
	"fmt"
	"log"
	"strings"

	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
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

func CollectChanges(rMap map[string]*Report, g *Graph) (bool, string, error) {
	// Iterate through each chart
	// Copy chart.Dependency struct into currChart.Metadata.Dependencies
	// Set currChart values to true for new dependency
	// Disable existing dependencies' dependency
	// "root new deps ['b', 'd', 'a'], parent_look_up={'f': ['a'], 'd': ['a', 'b'], 'c': ['a']}" indicates
	// root Chart.yaml dependencies
	// should have deps [a,b,d], root's values.yaml a.d.enabled: False and b.d.enabled: False,
	// d.enabled: True (assuming dependency condition is d.enabled)
	changesToAdd := ""
	changed := false
	addedDeps := false
	valuesFileChanged := false
	for c, r := range rMap {
		currChart := g.CMap[c]
		parentsMap := make(map[string][]*chart.Dependency)
		currMap := make(map[string]interface{})
		// Iterate through all dependencies
		for _, dep := range r.FullDeps {
			if _, ok := r.LookUp[getChartHash(dep)]; !ok {
				continue
			}
			// dep can be found in lookup
			parents := r.LookUp[getChartHash(dep)]
			// create mapping of parent chart to metadata dependencies
			for _, p := range parents {
				parentsMap[getChartHash(p)] = p.Metadata.Dependencies
			}
			var d chart.Dependency
			dStructList := parents[0].Metadata.Dependencies
			for _, i := range dStructList {
				if getDepHash(i) == getChartHash(dep) {
					d = *i
					log.Printf("adding dep %s-%s to chart %s\n", d.Name, d.Version, currChart.Name())
					addedDeps = true
					currChart.Metadata.Dependencies = append(currChart.Metadata.Dependencies, &chart.Dependency{
						Name:       d.Name,
						Version:    d.Version,
						Repository: d.Repository,
						Condition:  fmt.Sprintf("%s.%s", d.Name, "enabled"),
						Alias:      d.Alias,
					})
					condition := d.Condition
					// This will modify values.yaml and lose any comments
					// Alternative approach could be to return error message to instruct users to make changes to Values.yaml
					found, _ := SetValues(currChart, true, condition, currMap)
					valuesFileChanged = valuesFileChanged || found

				}
			}
			for _, currChartDep := range currChart.Metadata.Dependencies {
				if _, ok := parentsMap[getDepHash(currChartDep)]; !ok {
					continue
				}
				pdeps := parentsMap[getDepHash(currChartDep)]
				pdepCond := ""
				for _, pdep := range pdeps {
					if getDepHash(pdep) == getChartHash(dep) {
						pdepCond = pdep.Condition
						// This will modify values.yaml and lose any comments
						var nameToUse string
						if currChartDep.Alias != "" {
							nameToUse = currChartDep.Alias
						} else {
							nameToUse = currChartDep.Name
						}
						found, _ := SetValues(currChart, false, fmt.Sprintf("%s.%s", nameToUse, pdepCond), currMap)
						valuesFileChanged = valuesFileChanged || found
					}
				}
			}
		}

		changed = valuesFileChanged && addedDeps
		if valuesFileChanged {
			b, err := yaml.Marshal(currMap)
			if err != nil {
				return changed, "", err
			}
			changesToAdd += fmt.Sprintf("Modify to %s/values.yaml to contain: \n%s\n", currChart.Name(), string(b))
		}
		if changed {
			_ = chartutil.SaveDir(currChart, "/tmp/new-charts")
		}
	}
	return changed, changesToAdd, nil
}

func parsePath(key string) []string { return strings.Split(key, ".") }

func SetValues(chart *chart.Chart, v interface{}, p string, currMap map[string]interface{}) (bool, map[string]interface{}) {
	values := chart.Values
	// log.Printf("setting %s of %s to %v\n", p, chart.Name(), v)
	paths := parsePath(p)
	// construct how map should look like
	var tmp map[string]interface{}
	tmp = currMap
	for i := 0; i < len(paths)-1; i++ {
		currPath := paths[i]
		if _, ok := tmp[currPath]; !ok {
			// if field does not exist, create new field
			tmp[currPath] = make(map[string]interface{})
		}
		tmp = tmp[currPath].(map[string]interface{})
	}
	tmp[paths[len(paths)-1]] = v

	//Check values if field is set correctly
	tmp = values
	needsChange := true
	for i := 0; i < len(paths)-1; i++ {
		currPath := paths[i]
		if _, ok := tmp[currPath]; !ok {
			// if field no present, shortcircuit immediately
			return needsChange, currMap
		}
		tmp = tmp[currPath].(map[string]interface{})
	}
	if val, ok := tmp[paths[len(paths)-1]]; ok && val == v {
		needsChange = false
	}
	return needsChange, currMap

}
