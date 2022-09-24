package checker

import (
	"fmt"

	"github.com/rs/zerolog/log"

	"helm.sh/helm/v3/pkg/chart"
)

type Change struct {
	// Pass by values so changes to 1 chart does not affect contents
	DepToAdd     chart.Dependency
	SourceCharts []ChartW
	DepHash      string
	To           *ChartW
}

func (c *Change) Log() {
	log.Debug().Msg(fmt.Sprintln("DepToAdd:", c.DepToAdd.Name, "DepHash:", c.DepHash, "To:", c.To.ChartHash))
	cNames := []string{}
	for _, cw := range c.SourceCharts {
		cNames = append(cNames, cw.ChartHash)
	}
	log.Printf("sourceCharts=%s", cNames)
}

func (c *Change) EnableDep(root *ChartW, modifiedValues map[string]interface{}, newDepList []*chart.Dependency) (bool, bool, []*chart.Dependency) {
	addDep := true
	d := c.DepToAdd
	for _, rootDep := range root.Metadata.Dependencies {
		if GetDepHash(rootDep) == c.DepHash {
			addDep = false
			break
		}
	}
	if addDep {
		log.Debug().Msgf("Add dep %s-%s to chart %s\n", c.DepToAdd.Name, c.DepToAdd.Version, root.Name())
		nameToUse := nameOrAlias(&d)
		newDep := &chart.Dependency{
			Name:       d.Name,
			Version:    d.Version,
			Repository: d.Repository,
			Condition:  fmt.Sprintf("%s.%s", nameToUse, "enabled"),
			Alias:      d.Alias,
			Enabled:    true,
		}
		root.Metadata.Dependencies = append(root.Metadata.Dependencies, newDep)
		newDepList = append(newDepList, newDep)
	}
	// Collect changes to instruct users to make changes to Values.yaml
	valuesChanged, _ := BuildValues(root, true, d.Condition, modifiedValues)

	// if dep to add is a installer job, add extra values helm values from any source
	if d.Name == JobChartName {
		source := c.SourceCharts[0]
		nameToUse := nameOrAlias(&d)
		valuesToAdd := source.Values[nameToUse].(map[string]interface{})[HelmChartKey]
		valuesChanged, _ = BuildValues(root, valuesToAdd, fmt.Sprintf("%s.%s", nameToUse, HelmChartKey), modifiedValues)
	}
	return addDep, valuesChanged, newDepList
}

func (c *Change) DisableGrandchildDep(root *ChartW, modifiedValues map[string]interface{}) bool {
	// Search for existing dependencies to disable grandchild dependencies
	valuesChanged := false
	for _, source := range c.SourceCharts {
		depCondition := ""
		rootDepName := ""
		for _, dep := range source.Metadata.Dependencies {
			toUse := GetDepHash(dep)
			if toUse != c.DepHash {
				continue
			}
			depCondition = dep.Condition
		}
		for _, rootDep := range root.Metadata.Dependencies {
			if GetDepHash(rootDep) != source.ChartHash {
				continue
			}
			rootDepName = nameOrAlias(rootDep)
		}
		needsChange, _ := BuildValues(root, false, fmt.Sprintf("%s.%s", rootDepName, depCondition), modifiedValues)

		valuesChanged = valuesChanged || needsChange
	}
	return valuesChanged
}
