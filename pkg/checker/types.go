package checker

import (
	"github.com/rs/zerolog/log"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

type ChartType int

const (
	Normal ChartType = iota
	GenericInstaller
)

type ChartW struct {
	*chart.Chart
	ChartHash string
	CType     ChartType
	DepsW     []*ChartW
	ParentW   *ChartW
}

func (c *ChartW) Log() {
	cNames := []string{}
	for _, cw := range c.DepsW {
		cNames = append(cNames, cw.ChartHash)
	}
	log.Debug().Msgf("chartHash=%s, Deps=%s", c.ChartHash, cNames)
}

// func (cW *ChartW) AddMetadataDepdency(d *MetadataDepW) {
// 	cW.MetaDeps = append(cW.MetaDeps, d)
// 	cW.Metadata.Dependencies = append(cW.Metadata.Dependencies, d.Dependency)
// }

// type MetadataDepW struct {
// 	*chart.Dependency
// 	DepHash string
// }

func getAliasDependency(charts []*chart.Chart, dep *chart.Dependency) *chart.Chart {
	for _, c := range charts {
		if c == nil {
			continue
		}
		if c.Name() != dep.Name {
			continue
		}
		if !chartutil.IsCompatibleRange(dep.Version, c.Metadata.Version) {
			continue
		}

		out := *c
		md := *c.Metadata
		out.Metadata = &md

		if dep.Alias != "" {
			md.Name = dep.Alias
		}
		return &out
	}
	return nil
}

func NewChartW(c *chart.Chart) (*ChartW, error) {
	var newChartFunc func(c *chart.Chart, p *ChartW) (*ChartW, error)
	newChartFunc = func(c *chart.Chart, p *ChartW) (*ChartW, error) {
		log.Debug().Msgf("Generating new chart for %s", c.Name())
		cW := ChartW{}
		cW.Chart = c
		cW.ChartHash = GetChartHash(c)
		if p != nil {
			cW.ParentW = p
		}

		// Process dependencies, to add Aliased Dependencies as charts to c.Dependencies()
		chartDependencies := []*chart.Chart{}
		for _, req := range c.Metadata.Dependencies {
			if chartDependency := getAliasDependency(c.Dependencies(), req); chartDependency != nil {
				chartDependencies = append(chartDependencies, chartDependency)
			}
			if req.Alias != "" {
				// Replace name with alias
				req.Name = req.Alias
			}
		}
		c.SetDependencies(chartDependencies...)
		for _, d := range c.Dependencies() {
			n, err := newChartFunc(d, &cW)
			if err != nil {
				return nil, err
			}
			cW.DepsW = append(cW.DepsW, n)
		}
		return &cW, nil
	}
	cW, err := newChartFunc(c, nil)
	if err != nil {
		return nil, err
	}
	cW.Log()
	return cW, nil
}
