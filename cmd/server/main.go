package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"runtime/trace"

	"github.com/clagraff/devoid/network"
	"github.com/clagraff/devoid/server"
	"github.com/clagraff/devoid/state"

	errs "github.com/go-errors/errors"
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
	//defer profile.Start(profile.CPUProfile).Stop()
	info := network.MakeConnInfo("localhost", 8080, network.MakeUUID(),
		cfg.CertPath, cfg.KeyPath)

	s := network.NewServer(info)
	closeFn, tunnels, err := s.Serve()
	if err != nil {
		if e, ok := err.(*errs.Error); ok {
			fmt.Println(e.ErrorStack())
			panic(e)
		}
		panic(err)
	}

	defer closeFn()

	bytes, err := ioutil.ReadFile(cfg.EntitiesPath)
	if err != nil {
		panic(err)
	}

	gameState := state.NewState()
	if err := gameState.FromBytes(bytes); err != nil {
		panic(err)
	}

	server.Serve(gameState, tunnels)
}

func main() {
	f, err := os.Create("trace.out")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	err = trace.Start(f)
	if err != nil {
		panic(err)
	}
	defer trace.Stop()

	if len(os.Args) != 2 {
		panic("must specify path to server config")
	}

	cfg := loadServerConfig(os.Args[1])
	run(cfg)
}
