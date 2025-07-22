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

type conditionExecutorImpl struct {
	valueManager ValueManager
}

func newConditionExecutorImpl(valueManager ValueManager) *conditionExecutorImpl {
	return &conditionExecutorImpl{valueManager: valueManager}
}

func (c *conditionExecutorImpl) Execute(condition string) (bool, error) {
	if condition == "" {
		return true, nil
	}

	matches := exprReg.FindAllString(condition, -1)
	parameters := make(map[string]any, len(matches)+1)
	parameters["nil"] = nil
	for i, match := range matches {
		v, _, err := c.valueManager.GetProperty(match[1:])
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
		return false, newConditionResultNotBoolError(condition)
	}

	return v, nil
}
