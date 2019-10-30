package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/clagraff/devoid/entities"
	"github.com/clagraff/devoid/network"
	"github.com/clagraff/devoid/server"
)

type serverConfig struct {
	CertPath string `json:"certPath"`
	KeyPath  string `json:"keyPath"`

	EntitiesPath string `json:"entitiesPath"`
}

func loadServerConfig(path string) serverConfig {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	cfg := serverConfig{}

	err = json.Unmarshal(bytes, &cfg)
	if err != nil {
		panic(err)
	}

	return cfg
}

func run(cfg serverConfig) {
	locker := entities.MakeLocker()
	if err := locker.FromJSONFile(cfg.EntitiesPath); err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}

	info := network.MakeConnInfo("localhost", 8080, network.MakeUUID(),
		cfg.CertPath, cfg.KeyPath)

	s := network.NewServer(info)
	closeFn, tunnels, err := s.Serve()
	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}

	defer closeFn()

	server.Serve(&locker, tunnels)
}

func main() {
	if len(os.Args) != 2 {
		panic("must specify path to server config")
	}

	cfg := loadServerConfig(os.Args[1])
	run(cfg)
}
