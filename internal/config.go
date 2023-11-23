package ioc

import (
	"flag"
	"github.com/spf13/viper"
)

var ConfigFile = "./config/application.yaml"

type ConfigFetcher interface {
	Load() error
	Fetch(expr string) (any, bool)
}

type ConfigFetcherImpl struct {
}

func NewConfigFetcher() *ConfigFetcherImpl {
	return &ConfigFetcherImpl{}
}

func (c *ConfigFetcherImpl) Load() error {
	if !flag.Parsed() {
		flag.Parse()
	}

	viper.SetConfigFile(ConfigFile)
	return viper.ReadInConfig()
}

func (c *ConfigFetcherImpl) Fetch(expr string) (any, bool) {
	val := viper.Get(expr)
	return val, val != nil
}
