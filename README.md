# Helm dependency checker

### TLDR
* `depchecker` will construct a graph using the dependencies of helm charts, starting from a root helm chart. It will then search for overlapping dependencies (meaning deps that will be installed more than once if the umbrella chart is deployed). 
* `depchecker` returns to stdout the changes needed to be made to each chart's `Chart.yaml` and `values.yaml`. 
* When no changes need to be made, it will print
```sh
2022/09/24 19:40:54 main.go:38: SUCCESS, NO CHANGES REQUIRED
```

* Clone git repo
* Generate sample umbrella charts using:
```sh
cat << EOF > /tmp/helm_struct.json
{
  "root": [
    "a",
    "b",
    "d"
  ],
  "a": [
    "c",
    "d"
  ],
  "b": [
    "d"
  ],
  "c": [
    "e",
    "f"
  ],
  "d": [
    "f"
  ],
  "e": [],
  "f": []
}
EOF
go run pkg/cmd/chart-generator/chart-generator.go

```

* Run dep-checker on helm chart dir to see output

```sh
go run pkg/cmd/depchecker.go -p ./test-charts/my-charts
```
