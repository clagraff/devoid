package main

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"time"

	"github.com/clagraff/devoid/client"
	"github.com/clagraff/devoid/intents"
	"github.com/clagraff/devoid/network"
	"github.com/clagraff/devoid/server"
	"github.com/clagraff/devoid/state"

	errs "github.com/go-errors/errors"
	uuid "github.com/satori/go.uuid"
)

var serverID uuid.UUID = network.MakeUUID()
var serverSecret []byte = []byte("serverSecret")

func doClient(clientID uuid.UUID) {
	info := network.MakeConnInfo("localhost", 8080, clientID, "/home/clagraff/selfsigned.crt", "")
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

	gameState := state.NewState()
	intentsQueue := make(chan intents.Intent, 100)

	entityID := uuid.FromStringOrNil("7e874935-c241-4a40-8c71-54ac6d6c3eff")
	client.Serve(entityID, gameState, tunnel, intentsQueue)
}

func doServer() {
	//defer profile.Start(profile.CPUProfile).Stop()
	info := network.MakeConnInfo("localhost", 8080, network.MakeUUID(),
		"/home/clagraff/selfsigned.crt", "/home/clagraff/selfsigned.key")

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

	bytes, err := ioutil.ReadFile("entities.json")
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
	rand.Seed(time.Now().UnixNano())

	go doServer()

	time.Sleep(1 * time.Second)

	doClient(uuid.FromStringOrNil("7e874935-c241-4a40-8c71-54ac6d6c3eff"))
}
