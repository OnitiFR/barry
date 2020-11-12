package server

import (
	"errors"
	"fmt"
	"time"

	"github.com/Knetic/govaluate"
)

// Container describes a storage Container
type Container struct {
	Name     string
	CostExpr *govaluate.EvaluableExpression
}

type tomlContainer struct {
	Name string
	Cost string
}

// NewContainersConfigFromToml return a list of Container based on TOML [[container]] settings
func NewContainersConfigFromToml(tContainers []*tomlContainer) ([]*Container, error) {
	if len(tContainers) == 0 {
		return nil, errors.New("you must provide a least one [[container]] config")
	}

	containers := make([]*Container, 0)

	for _, tContainer := range tContainers {
		if tContainer.Name == "" {
			return nil, errors.New("container must have a 'name' setting")
		}

		if tContainer.Cost == "" {
			tContainer.Cost = "0"
		}

		expr, err := govaluate.NewEvaluableExpression(tContainer.Cost)
		if err != nil {
			return nil, fmt.Errorf("error in container cost expression: %s", err)
		}

		container := &Container{
			Name:     tContainer.Name,
			CostExpr: expr,
		}

		_, err = container.Cost(1, time.Second)
		if err != nil {
			return nil, fmt.Errorf("error in container cost expression: %s", err)
		}

		containers = append(containers, container)
	}
	return containers, nil
}

// Cost of a file in this container, based on its size and storage duration
func (c *Container) Cost(size int64, duration time.Duration) (float64, error) {
	params := make(map[string]interface{})

	params["size"] = size
	params["size_KB"] = float64(size) / 1024
	params["size_MB"] = float64(size) / 1024 / 1024
	params["size_GB"] = float64(size) / 1024 / 1024 / 1024
	params["size_TB"] = float64(size) / 1024 / 1024 / 1024 / 1024

	params["duration_secs"] = duration.Seconds()
	params["duration_hours"] = duration.Hours()
	params["duration_days"] = duration.Hours() / 24
	params["duration_months"] = duration.Hours() / 24 / 30
	params["duration_years"] = duration.Hours() / 24 / 365

	cost, err := c.CostExpr.Evaluate(params)
	if err != nil {
		return 0.0, err
	}

	costFloat, ok := cost.(float64)
	if !ok {
		return 0.0, errors.New("cannot convert result to a numeric value")
	}

	return costFloat, nil
}
