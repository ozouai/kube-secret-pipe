package kubesecretpipe

import (
	"context"
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Targets []*ConfigTarget `yaml:"targets"`
}

type ConfigTarget struct {
	BaseConfigMap   *InputConfigMap         `yaml:"inputConfigMap"`
	Secrets         map[string]*InputSecret `yaml:"secrets"`
	TargetNamespace string                  `yaml:"targetNamespace"`
	TargetName      string                  `yaml:"targetName"`
}

type InputSecret struct {
	Name      string   `yaml:"name"`
	Namespace string   `yaml:"namespace"`
	Keys      []string `yaml:"keys"`
}

type InputConfigMap struct {
	Namespace string `yaml:"namespace"`
	Name      string `yaml:"name"`
}

func ParseConfigFile(ctx context.Context, configFile string) (*Config, error) {
	configData, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("error reading config file '%s': %w", configFile, err)
	}

	baseConfig := &Config{}
	err = yaml.Unmarshal(configData, &baseConfig)
	if err != nil {
		return nil, fmt.Errorf("error parsing YAML data for config file: %w", err)
	}
	return baseConfig, nil
}
