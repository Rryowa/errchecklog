package main

import (
	"golang.org/x/tools/go/analysis"

	"github.com/Rryowa/errchecklog"
)

func New(conf any) ([]*analysis.Analyzer, error) {
	return errchecklog.New(conf)
}
