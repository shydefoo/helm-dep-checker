package checker

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/golang-collections/collections/queue"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

type Report struct {
	FullDeps []*ChartW

	Changes []*Change
}

type Checker struct {
	ChartDir string
	// RootChartName, RootChartVer string
	OverwriteChanges bool
}

func (checker *Checker) CollectChanges(rMap map[*ChartW]*Report, g *Graph) (bool, string, error) {
	// Iterate through each chart
	// Copy chart.Dependency struct into currChart.Metadata.Dependencies
	// Set currChart values to true for new dependency
	// Disable existing dependencies' dependency
	changesToAdd := ""
	chartsModified := false
	addedDeps := false
	valuesFileChanged := false
	for root, report := range rMap {
		modifiedValues := make(map[string]interface{})
		newDepList := []*chart.Dependency{}
		// Iterate through all dependencies
		for _, change := range report.Changes {
			var x, y bool
			x, y, newDepList = change.EnableDep(root, modifiedValues, newDepList)
			addedDeps = addedDeps || x
			valuesFileChanged = valuesFileChanged || y
			y = change.DisableGrandchildDep(root, modifiedValues)
			valuesFileChanged = valuesFileChanged || y
		}
		chartsModified = valuesFileChanged || addedDeps

		if addedDeps {
			b, err := yaml.Marshal(newDepList)
			if err != nil {
				return chartsModified, "", err
			}
			changesToAdd += fmt.Sprintf("Dependencies to add to %s/Chart.yaml: \n%s\n", root.Name(), string(b))
		}
		if valuesFileChanged {
			b, err := yaml.Marshal(modifiedValues)
			if err != nil {
				return chartsModified, "", err
			}
			changesToAdd += fmt.Sprintf("Modify to %s/values.yaml to contain: \n%s\n", root.Name(), string(b))
		}
		if chartsModified && checker.OverwriteChanges {
			_ = chartutil.SaveDir(root.Chart, checker.ChartDir)
		}
	}
	return chartsModified, changesToAdd, nil
}

func WalkGraph(g *Graph) map[*ChartW]*Report {
	log.Debug().Msg("walkgraph")
	report := make(map[*ChartW]*Report)

	var traverse func(c *ChartW) []*ChartW
	traverse = func(c *ChartW) []*ChartW {
		log.Debug().Msgf("traversing %s", c.Name())
		currentChash := c.ChartHash

		// Stores existing dependencies and new dependencies from grandchild charts
		depList := []*ChartW{}
		commonDep := []*ChartW{}
		q := queue.New()
		childToSourceM := make(map[string]map[string]*ChartW)

		newDepsFound := false
		// stores mapping between child chart and list of ancestor charts
		existingDeps := g.GMap[currentChash]
		for _, d := range existingDeps {
			q.Enqueue(d)
			log.Debug().Msgf("enqueue %s", d.ChartHash)
		}
		if len(existingDeps) == 0 {
			log.Debug().Msgf("done traversing: %s, no children", c.Name())
			return depList
		}
		depSet := make(map[string]*ChartW)
		for q.Len() > 0 {
			d := q.Dequeue().(*ChartW)
			log.Debug().Msgf("dequeue %s", d.ChartHash)
			if _, ok := depSet[d.ChartHash]; !ok {
				depSet[d.ChartHash] = d
			}
			grandChildDeps := traverse(d)
			for _, dGrand := range grandChildDeps {
				if _, ok := childToSourceM[dGrand.ChartHash]; !ok {
					childToSourceM[dGrand.ChartHash] = map[string]*ChartW{d.ChartHash: d}
				} else {
					// childToSourceM[dGrand.ChartHash] = append(childToSourceM[dGrand.ChartHash], d)
					tmp := childToSourceM[dGrand.ChartHash]
					if _, ok := tmp[d.ChartHash]; !ok {
						tmp[d.ChartHash] = d
					}
					commonDep = append(commonDep, dGrand)
					if _, ok := depSet[dGrand.ChartHash]; !ok {
						depSet[dGrand.ChartHash] = dGrand
						log.Debug().Msgf("New Deps found: %s for %s", dGrand.ChartHash, c.ChartHash)
						newDepsFound = true
						log.Debug().Msgf("enqueue %s", dGrand.ChartHash)
						q.Enqueue(dGrand)
					}
				}
			}
		}
		for k, v := range childToSourceM {
			log.Debug().Msgf("depMap Key=%s,parents:\n", k)
			for _, t := range v {
				t.Log()
			}
			log.Debug().Msgf("End of depMap Key=%s\n", k)
		}
		changes := []*Change{}
		// for depHash, depChartWSources := range depMap {
		for _, depChart := range commonDep {
			depHash := depChart.ChartHash
			depChartWSources := childToSourceM[depHash]
			change := Change{DepHash: depHash, To: c}
			var depToAdd chart.Dependency
			if depChart.ParentW == nil {
				continue
			}
			for _, d := range depChart.ParentW.Metadata.Dependencies {
				if d == nil {
					continue
				}
				if GetDepHash(d) != depHash {
					continue
				}
				depToAdd = *d
				break
			}
			for _, source := range depChartWSources {
				change.SourceCharts = append(change.SourceCharts, *source)
			}
			change.DepToAdd = depToAdd
			change.Log()
			changes = append(changes, &change)
		}
		for _, v := range depSet {
			depList = append(depList, v)
		}
		// Update chart's dependencies in graph
		g.GMap[currentChash] = depList
		if newDepsFound {
			report[c] = &Report{FullDeps: depList, Changes: changes}
		}
		log.Debug().Msgf("done traversing %s", c.Name())
		return depList
	}

	for k, v := range g.RMap {
		if v {
			log.Debug().Msgf("traversing %s", k)
			_ = traverse(g.CMap[k])
		}
	}

	return report
}
