package checker

import "helm.sh/helm/v3/pkg/chart"

type JobValues struct {
	Repository string `json:"repository"`
	Chart      string `json:"chart"`
	Version    string `json:"version"`
	Namespace  string `json:"namespace"`
	Release    string `json:"release"`
}

type CaramlChart interface {
	GetChartHash() string
	GetDepHash() string
	Dependencies() []*chart.Chart
}

// JobChart is a special chart that will create helm releases using a k8s job
type JobChart struct {
	*chart.Chart
	*JobValues
}

type DefaultChart struct {
	*chart.Chart
}

func (d *DefaultChart) GetChartHash() string {
	panic("not implemented") // TODO: Implement
}

func (d *DefaultChart) GetDepHash() string {
	panic("not implemented") // TODO: Implement
}

func (d *DefaultChart) Dependencies() []*chart.Chart {
	panic("not implemented") // TODO: Implement
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
