package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

var srcChartPath = "/Users/shide.foo/repos/shide/caraml-dep-checker/test-charts/foo"

func GenerateChart(input map[string][]string, root string, destPath string) (string, error) {
	var makeCharts func(string) (map[string]string, error)
	log.Println("Generate chart")
	path, err := os.MkdirTemp(destPath, "test_chart_*")
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	makeCharts = func(cname string) (map[string]string, error) {
		depList := input[cname]
		cfile := &chart.Metadata{
			Name:        cname,
			Description: "A Helm chart for Kubernetes",
			Type:        "application",
			Version:     "0.1.0",
			AppVersion:  "0.1.0",
			APIVersion:  chart.APIVersionV2,
		}
		deps := []*chart.Dependency{}
		for _, d := range depList {
			depMap, err := makeCharts(d)
			if err != nil {
				return nil, err
			}
			deps = append(deps, &chart.Dependency{Name: d, Repository: fmt.Sprintf("file://%s", depMap[d]), Version: "0.1.0", Condition: fmt.Sprintf("%s.enabled", d)})
		}
		cfile.Dependencies = deps
		cp := filepath.Join(path, cname)
		err = chartutil.CreateFrom(cfile, path, srcChartPath)
		if err != nil {
			return nil, err
		}
		return map[string]string{
			cname: cp,
		}, nil
	}
	_, err = makeCharts(root)
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	log.Println("Charts dir", path)
	return path, nil
}

func main() {
	x := make(map[string][]string)
	b, err := ioutil.ReadFile("/tmp/helm_struct.json")
	if err != nil {
		log.Fatal(err)
		panic(err)
	}
	if err := json.Unmarshal(b, &x); err != nil {
		log.Fatal(err)
		panic(err)
	}
	_, err = GenerateChart(x, "root", "/tmp/test-charts")
	if err != nil {
		panic(err)
	}
}
