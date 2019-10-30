package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/clagraff/devoid/client"
	"github.com/clagraff/devoid/entities"
	"github.com/clagraff/devoid/intents"
	"github.com/clagraff/devoid/network"

	errs "github.com/go-errors/errors"
	uuid "github.com/satori/go.uuid"
)

type clientConfig struct {
	CertPath string `json:"certPath"`
	ClientID uuid.UUID
	EntityID uuid.UUID
}

func loadClientConfig(path string) clientConfig {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	cfg := clientConfig{}

	err = json.Unmarshal(bytes, &cfg)
	if err != nil {
		panic(err)
	}

	return cfg
}

func run(cfg clientConfig) {
	info := network.MakeConnInfo("localhost", 8080, cfg.ClientID, cfg.CertPath, "")
	c := network.NewClient(info)
	closeFn, tunnel, err := c.Dial()
	if err != nil {
		if e, ok := err.(*errs.Error); ok {
			fmt.Println(e.ErrorStack())
			return
		}
		panic(err)
	}
	defer closeFn()

	locker := entities.MakeLocker()
	intentsQueue := make(chan intents.Intent, 100)

	client.Serve(cfg.EntityID, &locker, tunnel, intentsQueue)
}

func main() {
	if len(os.Args) != 2 {
		panic("must specify client JSON path")
	}

	cfg := loadClientConfig(os.Args[1])
	run(cfg)
}
