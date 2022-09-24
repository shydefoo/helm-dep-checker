package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chartutil"
)

var srcChartPath = flag.String("s", "./test-charts/foo", "source chart path")
var dagPath = flag.String("dag", "/tmp/helm_struct.json", "Path to helm chart graph definition")
var path = flag.String("dest", "./test-charts/my-charts", "Destination directory")

func GenerateChart(srcChartPath string, input map[string][]string, root string, destPath string) (string, error) {
	var makeCharts func(string) (map[string]string, error)
	log.Println("Generate chart")
	err := os.MkdirAll(destPath, 0744)
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
		cp := filepath.Join(destPath, cname)
		err = chartutil.CreateFrom(cfile, destPath, srcChartPath)
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
	log.Println("Charts dir", destPath)
	return destPath, nil
}

func main() {
	flag.Parse()
	x := make(map[string][]string)
	b, err := ioutil.ReadFile(*dagPath)
	if err != nil {
		log.Fatal(err)
		panic(err)
	}
	if err := json.Unmarshal(b, &x); err != nil {
		log.Fatal(err)
		panic(err)
	}
	fullPath, err := filepath.Abs(*path)
	if err != nil {
		panic(err)
	}
	if _, err := os.Stat(fullPath); err == nil {
		os.Remove(fullPath)
	}
	_, err = GenerateChart(*srcChartPath, x, "root", fullPath)
	if err != nil {
		panic(err)
	}
}
