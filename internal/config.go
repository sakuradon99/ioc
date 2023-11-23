package ioc

import (
	"flag"
	"github.com/spf13/viper"
)

var configFile = flag.String("ioc.config", "./config/application.yaml", "config file path")

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

	viper.SetConfigFile(*configFile)
	return viper.ReadInConfig()
}

func (c *ConfigFetcherImpl) Fetch(expr string) (any, bool) {
	val := viper.Get(expr)
	return val, val != nil
}
