package main

import (
	"github.com/mitchellh/mapstructure"
	"golang.org/x/tools/go/analysis"
)

type Config struct {
	InterfacePackage string `mapstructure:"interface_package"`
	InterfaceName    string `mapstructure:"interface_name"`
}

func New(conf any) ([]*analysis.Analyzer, error) {
	var cfg Config
	if err := mapstructure.Decode(conf, &cfg); err != nil {
		return nil, err
	}
	analyzer := NewAnalyzer(cfg.InterfacePackage, cfg.InterfaceName)
	return []*analysis.Analyzer{analyzer}, nil
}
