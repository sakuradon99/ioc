package ioc

import (
	"fmt"
	"github.com/Knetic/govaluate"
	"regexp"
	"strings"
)

var exprReg = regexp.MustCompile(`#([a-zA-Z0-9_.]+)`)

type ConditionExecutor interface {
	Execute(condition string) (bool, error)
}

type ConditionExecutorImpl struct {
	sourceManager PropertyManager
}

func NewConditionExecutorImpl(sourceManager PropertyManager) *ConditionExecutorImpl {
	return &ConditionExecutorImpl{sourceManager: sourceManager}
}

func (c *ConditionExecutorImpl) Execute(condition string) (bool, error) {
	if condition == "" {
		return true, nil
	}

	matches := exprReg.FindAllString(condition, -1)
	parameters := make(map[string]any, len(matches)+1)
	parameters["nil"] = nil
	for i, match := range matches {
		v, _, err := c.sourceManager.GetProperty(match[1:])
		if err != nil {
			return false, err
		}

		p := fmt.Sprintf("p%d", i)
		parameters[p] = v
		condition = strings.Replace(condition, match, p, 1)
	}

	expression, err := govaluate.NewEvaluableExpression(condition)
	if err != nil {
		return false, err
	}
	result, err := expression.Evaluate(parameters)
	if err != nil {
		return false, err
	}
	v, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("condition <%s> should return bool", condition)
	}

	return v, nil
}
