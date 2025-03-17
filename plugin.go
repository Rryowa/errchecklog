package main

import (
	"golang.org/x/tools/go/analysis"
)

type Plugin struct{}

var PluginEntry = Plugin{}

// ConfigFunc creates a default config instance that GolangCI-Lint will fill from `.golangci-lint.yaml`.
func (p *Plugin) ConfigFunc() interface{} {
	// Return a pointer to a default config.
	return &Config{
		InterfacePackage: "",
		InterfaceName:    "",
	}
}

// Run is called after GolangCI-Lint populates the config.
// We create and return the analyzers to run, using the config values.
func (p *Plugin) Run(cfg interface{}) (checks []*analysis.Analyzer, err error) {
	realCfg := cfg.(*Config)
	analyzer := NewAnalyzer(realCfg.InterfacePackage, realCfg.InterfaceName)
	return []*analysis.Analyzer{analyzer}, nil
}
