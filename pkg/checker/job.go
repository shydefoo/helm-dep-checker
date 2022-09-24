package checker

import (
	"errors"

	"helm.sh/helm/v3/pkg/chart"
)

func GetJobInfoForDep(ownerChart *chart.Chart, d *chart.Dependency) (*JobValues, error) {
	vals := ownerChart.Values
	nameToUse := nameOrAlias(d)
	var jv *JobValues
	var hc map[string]interface{}
	if values, ok := vals[nameToUse]; ok {
		jv = &JobValues{}
		val := values.(map[string]interface{})
		hc, _ = val["helmChart"].(map[string]interface{})
		if v, ok := hc["repository"]; ok {
			jv.Repository = v.(string)
		}
		if v, ok := hc["chart"]; ok {
			jv.Chart = v.(string)
		}
		if v, ok := hc["version"]; ok {
			jv.Version = v.(string)
		}
		if v, ok := hc["namespace"]; ok {
			jv.Namespace = v.(string)
		}
		if v, ok := hc["release"]; ok {
			jv.Release = v.(string)
		}
	}
	if jv == nil {
		return nil, errors.New("Job is nil")
	}
	return jv, nil
}
