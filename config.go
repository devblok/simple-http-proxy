package main

import (
	"encoding/json"
	"os"
)

type Config struct {
	Users []UserConfig `json:"users"`
}

type UserConfig struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	QuotaBytes uint   `json:"quota_bytes"`
}

func loadConfig(path string) (config Config, err error) {
	file, err := os.Open(path)
	if err != nil {
		return
	}

	err = json.NewDecoder(file).Decode(&config)
	return
}
