package checker

import (
	"github.com/rs/zerolog/log"
	"helm.sh/helm/v3/pkg/chart"
)

type graphM map[string][]*ChartW

type chartM map[string]*ChartW
type Graph struct {
	GMap graphM
	CMap chartM
	RMap map[string]bool
}

const (
	JobChartName = "generic-dep-installer"
	HelmChartKey = "helmChart"
)

func IsGenericInstaller(c *chart.Chart) bool {
	return c.Name() == JobChartName
}

func ConstructGraph(charts []*ChartW) (*Graph, error) {
	g := make(map[string][]*ChartW)
	cM := make(map[string]*ChartW)
	rM := make(map[string]bool)
	var _makeGraph func(*ChartW)
	_makeGraph = func(c *ChartW) {
		chash := c.ChartHash
		if _, ok := cM[chash]; !ok {
			cM[chash] = c
		}
		if _, ok := g[chash]; !ok {
			// TODO: filter for charts that are NOT library types
			g[chash] = c.DepsW
			for _, d := range c.DepsW {
				rM[d.ChartHash] = false
				_makeGraph(d)
			}
		}
	}
	for _, c := range charts {
		// set all charts to be roots
		rM[c.ChartHash] = true
	}
	for _, c := range charts {
		// Only construct graph for root charts, dependency charts will get added in through MakeGraph
		_makeGraph(c)
	}
	graph := &Graph{
		GMap: g, CMap: cM, RMap: rM,
	}
	graph.PrintGraph()
	return graph, nil
}

func (g *Graph) PrintGraph() {
	for k, v := range g.GMap {
		cNames := []string{}
		for _, cw := range v {
			cNames = append(cNames, cw.ChartHash)
		}
		log.Debug().Msgf("node: %s, edges: %s", k, cNames)
	}
}
