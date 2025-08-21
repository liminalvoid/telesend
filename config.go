package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const cfgFolder = ".telesend"

type Auth struct {
	AppID       int    `toml:"app-id"`
	AppHash     string `toml:"app-hash"`
	PhoneNumber string `toml:"phone-number"`
}

type General struct {
	ContactsPerPage int `toml:"contacts-per-page"`
}

type Misc struct {
	CheckboxSelected string `toml:"checkbox-selected"`
	CheckboxClear    string `toml:"checkbox-clear"`
	Cursor           string `toml:"cursor"`
}

type Config struct {
	Auth    Auth
	General General
	Misc    Misc
}

func createConfig(cfg *Config) error {
	// Defaults
	cfg.General.ContactsPerPage = 10
	cfg.Misc.CheckboxSelected = "■"
	cfg.Misc.CheckboxClear = "□"
	cfg.Misc.Cursor = "▣"

	b, err := toml.Marshal(*cfg)
	if err != nil {
		return err
	}

	// Creating config folder
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(homeDir, cfgFolder), 0700); err != nil {
		return err
	}

	// Make config absolute path and write config to file
	cfgPath := filepath.Join(homeDir, cfgFolder, "config.toml")
	err = os.WriteFile(cfgPath, b, 0600)
	if err != nil {
		return err
	}

	fmt.Printf("Config saved in %s\n", cfgPath)

	return nil
}

func readConfig(cfg *Config) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(homeDir, cfgFolder, "config.toml")

	cfgFile, err := os.ReadFile(cfgPath)
	if err != nil {
		return err
	}

	err = toml.Unmarshal(cfgFile, cfg)
	if err != nil {
		return err
	}

	return nil
}
