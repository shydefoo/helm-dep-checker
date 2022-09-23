package checker

import (
	"fmt"
	"io"
	"log"
	"os"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
)

type JobValues struct {
	Repository string `json:"repository"`
	Chart      string `json:"chart"`
	Version    string `json:"version"`
	Namespace  string `json:"namespace"`
	Release    string `json:"release"`
}
type ChartType int

const (
	Normal ChartType = iota
	GenericInstaller
)

type ChartW struct {
	*chart.Chart
	ChartHash string
	CType     ChartType
	Job       *JobValues
	DepsW     []*ChartW
}

func NewChartW(c *chart.Chart) (*ChartW, error) {
	var newChartFunc func(c *chart.Chart) (*ChartW, error)
	newChartFunc = func(c *chart.Chart) (*ChartW, error) {
		cW := ChartW{}
		cW.Chart = c
		var err error
		if IsGenericInstaller(c) {
			cW.CType = GenericInstaller
			cW.ChartHash, cW.Job, err = getGenInstallerHashStrict(c)
			if err != nil {
				return nil, err
			}
		} else {
			cW.CType = Normal
			cW.ChartHash = getChartHash(c)
		}
		for _, d := range c.Dependencies() {
			n, err := newChartFunc(d)
			if err != nil {
				return nil, err
			}
			cW.DepsW = append(cW.DepsW, n)
		}
		return &cW, nil
	}
	cW, err := newChartFunc(c)
	if err != nil {
		return nil, err
	}
	return cW, nil
}

var settings = cli.New()

func debug(format string, v ...interface{}) {
	if settings.Debug {
		format = fmt.Sprintf("[debug] %s\n", format)
		_ = log.Output(2, fmt.Sprintf(format, v...))
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
