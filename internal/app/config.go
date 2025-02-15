package app

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	CfgDB        ConfigDB `yaml:"db"`
	MaxOpenConns int      `yaml:"max_open_conns"`
	Secret       string   `yaml:"secret"`
	ServerPort   string   `yaml:"srv_port"`
}

type ConfigDB struct {
	Login    string `yaml:"login"`
	Password string `yaml:"password"`
	Port     uint   `yaml:"port"`
	Database string `yaml:"database"`
	Host     string `yaml:"host"`
}

func NewConfig(configPath string) (*Config, error) {
	cfg, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var c Config
	err = yaml.Unmarshal(cfg, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}
