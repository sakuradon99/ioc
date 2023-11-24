package ioc

import (
	"github.com/spf13/viper"
)

var ConfigFile = "./config/application.yaml"

type ConfigFetcher interface {
	Fetch(expr string) (any, bool, error)
}

type ConfigFetcherImpl struct {
	loaded bool
}

func NewConfigFetcher() *ConfigFetcherImpl {
	return &ConfigFetcherImpl{}
}

func (c *ConfigFetcherImpl) Fetch(expr string) (any, bool, error) {
	if !c.loaded {
		viper.SetConfigFile(ConfigFile)
		err := viper.ReadInConfig()
		if err != nil {
			return nil, false, err
		}
		c.loaded = true
	}

	val := viper.Get(expr)
	return val, val != nil, nil
}
