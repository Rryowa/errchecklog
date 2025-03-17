package main

type Config struct {
	InterfacePackage string `mapstructure:"interface_package"`
	InterfaceName    string `mapstructure:"interface_name"`
}
