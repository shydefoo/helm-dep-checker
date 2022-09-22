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

// GetDependencyObj returns matching dependency chart from parent charts Metadata.Dependency
// field. Only applies to immediate parent of chart
func GetDependencyObj(c *chart.Chart) *chart.Dependency {
	parent := c.Parent()
	for _, d := range parent.Metadata.Dependencies {
		if d.Name == c.Name() {
			return d
		}
	}
	return nil
}
