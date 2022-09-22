package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"path/filepath"

	"os"

	"caraml-dev/caraml-dep-checker/pkg/checker"

	"golang.org/x/sync/errgroup"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
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

func main() {
	c, err := getCharts("/tmp/test-charts/test_chart_3478466454")
	if err != nil {
		panic(err)
	}
	g, err := checker.ConstructGraph(c)
	if err != nil {
		panic(err)
	}
	report := checker.WalkGraph(g, g.CMap["root-0.1.0"])
	changed, changes, err := checker.CollectChanges(report, g)
	if err != nil {
		panic(err)
	}
	if changed {
		log.Println("Changes required:")
		log.Fatal(changes)
		os.Exit(1)
	}

}
