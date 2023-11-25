package ioc

import (
	"github.com/spf13/viper"
)

var SourceFile = "./config/application.yaml"

type SourceManager interface {
	GetValue(expr string) (any, bool, error)
}

type SourceManagerImpl struct {
	loaded bool
}

func NewSourceManagerImpl() *SourceManagerImpl {
	return &SourceManagerImpl{}
}

func (c *SourceManagerImpl) GetValue(expr string) (any, bool, error) {
	if !c.loaded {
		viper.SetConfigFile(SourceFile)
		err := viper.ReadInConfig()
		if err != nil {
			return nil, false, err
		}
		c.loaded = true
	}

	val := viper.Get(expr)
	return val, val != nil, nil
}
