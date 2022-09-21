package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"

	"os"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
)

var settings = cli.New()

func debug(format string, v ...interface{}) {
	if settings.Debug {
		format = fmt.Sprintf("[debug] %s\n", format)
		log.Output(2, fmt.Sprintf(format, v...))
	}
}

func setup(chartpath string, out io.Writer) (*downloader.Manager, error) {

	actionConfig := new(action.Configuration)
	helmDriver := os.Getenv("HELM_DRIVER")
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), helmDriver, debug); err != nil {
		log.Fatal(err)
	}
	client := action.NewDependency()
	man := &downloader.Manager{
		Out:              out,
		ChartPath:        chartpath,
		Keyring:          client.Keyring,
		SkipUpdate:       client.SkipRefresh,
		Getters:          getter.All(settings),
		RegistryClient:   actionConfig.RegistryClient,
		RepositoryConfig: settings.RepositoryConfig,
		RepositoryCache:  settings.RepositoryCache,
		Debug:            settings.Debug,
	}
	return man, nil

}

func getCharts(chartDir string) ([]*chart.Chart, error) {
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

type graphM map[string][]*chart.Chart
type chartM map[string]*chart.Chart

type Graph struct {
	GMap graphM
	CMap chartM
}

type Report struct {
	FullDeps []*chart.Chart
	LookUp   map[string][]*chart.Chart
}

func getChartHash(c *chart.Chart) string {
	return fmt.Sprintf("%s-%s", c.Name(), c.Metadata.Version)
}

func getDepHash(d *chart.Dependency) string {
	return fmt.Sprintf("%s-%s", d.Name, d.Version)
}

func constructGraph(charts []*chart.Chart) (*Graph, error) {
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

func getDepReport(g *Graph, rootChart *chart.Chart) map[string]*Report {

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

func ModifyYaml(rMap map[string]*Report, g *Graph) {
	// Iterate through each chart
	// Copy chart.Dependency struct into currChart.Metadata.Dependencies
	// Set currChart values to true for new dependency
	// Disable existing dependencies' dependency
	// "root new deps ['b', 'd', 'a'], parent_look_up={'f': ['a'], 'd': ['a', 'b'], 'c': ['a']}" indicates
	// root Chart.yaml dependencies
	// should have deps [a,b,d], root's values.yaml a.d.enabled: False and b.d.enabled: False,
	// d.enabled: True (assuming dependency condition is d.enabled)
	for c, r := range rMap {
		currChart := g.CMap[c]
		parentsMap := make(map[string][]*chart.Dependency)
		for _, dep := range r.FullDeps {
			// dep can be found in lookup
			if parents, ok := r.LookUp[getChartHash(dep)]; ok {
				// copy Dependency object over
				for _, p := range parents {
					parentsMap[getChartHash(p)] = p.Metadata.Dependencies
				}
				var d chart.Dependency
				dStructList := parents[0].Metadata.Dependencies
				for _, i := range dStructList {
					if getDepHash(i) == getChartHash(dep) {
						d = *i
						log.Printf("adding dep %s-%s to chart %s\n", d.Name, d.Version, currChart.Name())
						currChart.Metadata.Dependencies = append(currChart.Metadata.Dependencies, &chart.Dependency{
							Name:       d.Name,
							Version:    d.Version,
							Repository: d.Repository,
							Condition:  fmt.Sprintf("%s.%s", d.Name, "enabled"),
						})
						condition := d.Condition
						// This will modify values.yaml and lose any comments
						// Alternative approach could be to return error message to instruct users to make changes to Values.yaml
						SetValues(currChart, true, condition)
						break
					}
				}
				for _, currChartDep := range currChart.Metadata.Dependencies {
					if pdeps, ok := parentsMap[getDepHash(currChartDep)]; ok {
						pdepCond := ""
						for _, pdep := range pdeps {
							if getDepHash(pdep) == getChartHash(dep) {
								pdepCond = pdep.Condition
								// This will modify values.yaml and lose any comments
								SetValues(currChart, false, fmt.Sprintf("%s.%s", currChartDep.Name, pdepCond))
								break
							}
						}
					}
				}
			}
		}

		b, _ := yaml.Marshal(currChart.Values)
		// if err != nil {
		// 	return errors.Wrap(err, "reading values file")
		// }
		for _, f := range currChart.Raw {
			if f.Name == chartutil.ValuesfileName {
				f.Data = b
			}
		}
		_ = chartutil.SaveDir(currChart, "/tmp")
	}
}

func parsePath(key string) []string { return strings.Split(key, ".") }

func SetValues(chart *chart.Chart, v interface{}, p string) map[string]interface{} {
	values := chart.Values
	log.Printf("setting %s of %s to %v\n", p, chart.Name(), v)
	paths := parsePath(p)
	var currMap map[string]interface{}
	currMap = values
	for i := 0; i < len(paths)-1; i++ {
		currPath := paths[i]
		if _, ok := currMap[currPath]; !ok {
			currMap[currPath] = make(map[string]interface{})
		}
		currMap = currMap[currPath].(map[string]interface{})
	}
	currMap[paths[len(paths)-1]] = v
	log.Printf("values=%+v", values)

	return values
}

func main() {
	c, err := getCharts("/tmp/test-charts/test_chart_349618503")
	if err != nil {
		panic(err)
	}
	g, err := constructGraph(c)
	if err != nil {
		panic(err)
	}
	report := getDepReport(g, g.CMap["root-0.1.0"])
	ModifyYaml(report, g)

}
