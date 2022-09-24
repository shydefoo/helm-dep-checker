package checker

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

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
		_ = log.Output(2, fmt.Sprintf(format, v...))
	}
}

func Setup(chartpath string, out io.Writer) (*downloader.Manager, error) {

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
				m, err := Setup(chartPath, os.Stdout)
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
