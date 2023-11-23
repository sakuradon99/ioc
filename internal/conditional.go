package ioc

import (
	"fmt"
	"github.com/Knetic/govaluate"
	"github.com/spf13/viper"
	"regexp"
	"strings"
)

var exprReg = regexp.MustCompile(`#([a-zA-Z0-9_.]+)`)

type ConditionExecutor interface {
	Execute(condition string) (bool, error)
}

type ConditionExecutorImpl struct {
	configFetcher ConfigFetcher
}

func NewConditionExecutorImpl(configFetcher ConfigFetcher) *ConditionExecutorImpl {
	return &ConditionExecutorImpl{configFetcher: configFetcher}
}

func (c *ConditionExecutorImpl) Execute(condition string) (bool, error) {
	if condition == "" {
		return true, nil
	}

	matches := exprReg.FindAllString(condition, -1)
	parameters := make(map[string]any, len(matches)+1)
	parameters["nil"] = nil
	for i, match := range matches {
		v := viper.Get(match[1:])
		p := fmt.Sprintf("p%d", i)
		parameters[p] = v
		condition = strings.ReplaceAll(condition, match, p)
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
