package checker

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
)

type Checker struct {
	ChartDir                    string
	RootChartName, RootChartVer string
	OverwriteChanges            bool
}

func (checker *Checker) CollectChanges(rMap map[string]*Report, g *Graph) (bool, string, error) {
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
		depAncestorMap := make(map[string][]*chart.Dependency)
		modifiedValues := make(map[string]interface{})
		newDepList := []*chart.Dependency{}
		// Iterate through all dependencies
		for _, depC := range r.FullDeps {
			depAncestors, ok := r.LookUp[getChartHash(depC)]
			if !ok {
				continue
			}
			// create mapping of parent chart to metadata dependencies
			for _, p := range depAncestors {
				depAncestorMap[getChartHash(p)] = p.Metadata.Dependencies
			}

			var d chart.Dependency
			selectedParent := depAncestors[0]
			// Search through list of dependencies to find dependency to add to Parent currChart
			for _, i := range selectedParent.Metadata.Dependencies {
				if getDepHashFromParent(depC, i) != getChartHash(depC) {
					continue
				}
				d = *i
				log.Printf("adding dep %s-%s to chart %s\n", d.Name, d.Version, currChart.Name())
				addedDeps = true
				newDep := &chart.Dependency{
					Name:       d.Name,
					Version:    d.Version,
					Repository: d.Repository,
					Condition:  fmt.Sprintf("%s.%s", nameOrAlias(&d), "enabled"),
					Alias:      d.Alias,
					Enabled:    true,
				}
				currChart.Metadata.Dependencies = append(currChart.Metadata.Dependencies, newDep)
				newDepList = append(newDepList, newDep)
				// Collect changes to instruct users to make changes to Values.yaml
				found, _ := SetValues(currChart, true, d.Condition, modifiedValues)
				if IsGenericInstaller(depC) {
					helmChartV := depC.Parent().Values[nameOrAlias(&d)].(map[string]interface{})[HelmChartKey]
					found, _ = SetValues(currChart, helmChartV, fmt.Sprintf("%s.%s", nameOrAlias(&d), HelmChartKey), modifiedValues)
				}
				valuesFileChanged = valuesFileChanged || found
			}
			// Search for existing dependencies to disable grandchild dependencies
			for _, currChartDep := range currChart.Metadata.Dependencies {
				pdeps, ok := depAncestorMap[getDepHashFromParent(currChart, currChartDep)]
				if !ok {
					continue
				}
				pdepCond := ""
				for _, pdep := range pdeps {
					if getDepHashFromParent(depC, pdep) == getChartHash(depC) {
						pdepCond = pdep.Condition
						// This will modify values.yaml and lose any comments
						nameToUse := nameOrAlias(currChartDep)
						found, _ := SetValues(currChart, false, fmt.Sprintf("%s.%s", nameToUse, pdepCond), modifiedValues)
						valuesFileChanged = valuesFileChanged || found
					}
				}
			}
		}

		changed = valuesFileChanged || addedDeps
		if addedDeps {
			b, err := yaml.Marshal(newDepList)
			if err != nil {
				return changed, "", err
			}
			changesToAdd += fmt.Sprintf("Dependencies to add to %s/Chart.yaml: \n%s\n", currChart.Name(), string(b))
		}
		if valuesFileChanged {
			b, err := yaml.Marshal(modifiedValues)
			if err != nil {
				return changed, "", err
			}
			changesToAdd += fmt.Sprintf("Modify to %s/values.yaml to contain: \n%s\n", currChart.Name(), string(b))
		}
		if changed && checker.OverwriteChanges {
			_ = chartutil.SaveDir(currChart, checker.ChartDir)
		}
	}
	return changed, changesToAdd, nil
}

func GetCharts(chartDir string) ([]*chart.Chart, error) {
	// chartDir := "./test-charts"
	log.Println("Get charts")
	charts := []*chart.Chart{}
	files, err := ioutil.ReadDir(chartDir)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	var g errgroup.Group
	for _, file := range files {
		if file.IsDir() {
			chartPath := filepath.Join(chartDir, file.Name())
			log.Println("chartPath", chartPath)
			// dir is a helm chart
			g.Go(func() error {
				m, err := setup(chartPath, os.Stdout)
				if err != nil {
					return err
				}
				// Download dependencies
				if err := m.Build(); err != nil {
					log.Fatal(err)
					return err
				}
				c, err := loader.Load(chartPath)
				if err != nil {
					log.Fatal(err)
					return err
				}
				charts = append(charts, c)
				return nil
			})
		}
	}
	if err := g.Wait(); err != nil {
		log.Fatal(err)
		return nil, err
	}
	return charts, nil
}
func parsePath(key string) []string { return strings.Split(key, ".") }
func nameOrAlias(d *chart.Dependency) string {
	if d.Alias != "" {
		return d.Alias
	}
	return d.Name
}

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
