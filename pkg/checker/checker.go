package checker

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
)

type Checker struct {
	ChartDir string
	// RootChartName, RootChartVer string
	OverwriteChanges bool
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
		root := g.CMap[c]
		depAncestorMap := make(map[string][]*MetadataDepW)
		modifiedValues := make(map[string]interface{})
		newDepList := []*chart.Dependency{}
		// Iterate through all dependencies
		for _, depC := range r.FullDeps {
			depAncestors, ok := r.LookUp[depC.ChartHash]
			if !ok {
				continue
			}
			// create mapping of parent chart to metadata dependencies
			for _, p := range depAncestors {
				depAncestorMap[depC.ChartHash] = p.MetaDeps
			}

			var d chart.Dependency
			selectedParent := depC.ParentW
			// Search through list of dependencies to find dependency to add to Parent currChart
			for _, i := range selectedParent.MetaDeps {
				if i.DepHash != depC.ChartHash {
					continue
				}
				d = *i.Dependency
				nameToUse := nameOrAlias(&d)
				newDep := &chart.Dependency{
					Name:       d.Name,
					Version:    d.Version,
					Repository: d.Repository,
					Condition:  fmt.Sprintf("%s.%s", nameToUse, "enabled"),
					Alias:      d.Alias,
					Enabled:    true,
				}
				addDep := true
				newMetaD := MetadataDepW{newDep, i.DepHash}

				// check if root.Metadata.Dependencies already has dependency added
				for _, rootDep := range root.MetaDeps {
					if rootDep.DepHash == i.DepHash {
						addDep = false
						break
					}
				}
				if addDep {
					log.Printf("Add dep %s-%s to chart %s\n", d.Name, d.Version, root.Name())
					// root.Metadata.Dependencies = append(root.Metadata.Dependencies, newDep)
					root.AddMetadataDepdency(&newMetaD)
					newDepList = append(newDepList, newDep)
					addedDeps = true
				}
				// Collect changes to instruct users to make changes to Values.yaml
				found, _ := BuildValues(root, true, d.Condition, modifiedValues)
				if depC.CType == GenericInstaller {
					helmChartV := depC.Parent().Values[nameToUse].(map[string]interface{})[HelmChartKey]
					found, _ = BuildValues(root, helmChartV, fmt.Sprintf("%s.%s", nameToUse, HelmChartKey), modifiedValues)
				}
				valuesFileChanged = valuesFileChanged || found
			}
			// Search for existing dependencies to disable grandchild dependencies
			for _, rootChartDep := range root.MetaDeps {
				pdeps, ok := depAncestorMap[rootChartDep.DepHash]
				if !ok {
					continue
				}
				for _, pdep := range pdeps {
					if pdep.DepHash == depC.ChartHash {
						nameToUse := nameOrAlias(rootChartDep.Dependency)
						found, _ := BuildValues(root, false, fmt.Sprintf("%s.%s", nameToUse, pdep.Condition), modifiedValues)
						valuesFileChanged = valuesFileChanged || found
						break
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
			changesToAdd += fmt.Sprintf("Dependencies to add to %s/Chart.yaml: \n%s\n", root.Name(), string(b))
		}
		if valuesFileChanged {
			b, err := yaml.Marshal(modifiedValues)
			if err != nil {
				return changed, "", err
			}
			changesToAdd += fmt.Sprintf("Modify to %s/values.yaml to contain: \n%s\n", root.Name(), string(b))
		}
		if changed && checker.OverwriteChanges {
			_ = chartutil.SaveDir(root.Chart, checker.ChartDir)
		}
	}
	return changed, changesToAdd, nil
}

func GetCharts(chartDir string) ([]*ChartW, error) {
	// chartDir := "./test-charts"
	log.Println("Get charts")
	charts := []*chart.Chart{}
	chartsW := []*ChartW{}
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

	for _, c := range charts {
		chartWrapper, err := NewChartW(c)
		if err != nil {
			return nil, err
		}
		chartsW = append(chartsW, chartWrapper)

	}
	return chartsW, nil
}
func parsePath(key string) []string { return strings.Split(key, ".") }
func nameOrAlias(d *chart.Dependency) string {
	if d.Alias != "" {
		return d.Alias
	}
	return d.Name
}

func BuildValues(chart *ChartW, v interface{}, p string, currMap map[string]interface{}) (bool, map[string]interface{}) {
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
			// if field no present, return immediately
			return needsChange, currMap
		}
		tmp = tmp[currPath].(map[string]interface{})
	}
	if val, ok := tmp[paths[len(paths)-1]]; ok && cmp.Equal(val, v) {
		needsChange = false
	}
	return needsChange, currMap

}
