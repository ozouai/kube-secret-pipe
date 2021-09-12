package main

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
