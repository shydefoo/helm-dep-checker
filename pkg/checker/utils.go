package checker

import (
	"fmt"
	"strings"

	"github.com/google/go-cmp/cmp"
	"helm.sh/helm/v3/pkg/chart"
)

func GetChartHash(c *chart.Chart) string {
	return fmt.Sprintf("%s-%s", c.Name(), c.Metadata.Version)
}

func GetDepHash(d *chart.Dependency) string {
	toUse := nameOrAlias(d)
	return fmt.Sprintf("%s-%s", toUse, d.Version)
}
func nameOrAlias(d *chart.Dependency) string {
	if d.Alias != "" {
		return d.Alias
	}
	return d.Name
}
func parsePath(key string) []string { return strings.Split(key, ".") }

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
	valuesChanged := true
	for i := 0; i < len(paths)-1; i++ {
		currPath := paths[i]
		if _, ok := tmp[currPath]; !ok {
			// if field no present, return immediately
			return valuesChanged, currMap
		}
		tmp = tmp[currPath].(map[string]interface{})
	}
	if val, ok := tmp[paths[len(paths)-1]]; ok && cmp.Equal(val, v) {
		valuesChanged = false
	}
	return valuesChanged, currMap

}
