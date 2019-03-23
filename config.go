package main

import (
	"fmt"
	"github.com/spf13/viper"
	"time"
)

type AppConfig struct {
	Port int
	Mode string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
	TTL      time.Duration
}

func getConfig(subKey string, config interface{}) error {
	var v *viper.Viper
	if v = viper.Sub(subKey); v == nil {
		panic("Fatal error config file missing " + subKey)
	} else {
		if err := v.Unmarshal(config); err != nil {
			return fmt.Errorf("Fatal error config file unmarshal %s: %s \n", subKey, err)
		}
	}
	return nil
}
