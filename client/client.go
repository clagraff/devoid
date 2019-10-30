package client

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/clagraff/devoid/components"
	"github.com/clagraff/devoid/entities"
	"github.com/clagraff/devoid/intents"
	"github.com/clagraff/devoid/mutators"
	"github.com/clagraff/devoid/network"

	termbox "github.com/nsf/termbox-go"
	uuid "github.com/satori/go.uuid"
)

type Cursor struct {
	X        int
	Y        int
	nextSwap time.Time
	state    bool
}

func (cursor *Cursor) Render() {
	if time.Now().After(cursor.nextSwap) {
		cursor.state = !cursor.state
		cursor.nextSwap = time.Now().Add(250 * time.Millisecond)
	}

	ch := '▮'
	if !cursor.state {
		ch = '▯'
	}
	termbox.SetCell(cursor.X, cursor.Y, ch, termbox.ColorYellow, termbox.ColorBlack)
}

var c *Cursor = new(Cursor)

func init() {
	c.X = 12
	c.Y = 13
}

type direction int

const (
	up direction = iota
	right
	down
	left
)

func Serve(entityID uuid.UUID, locker *entities.Locker, tunnel network.Tunnel, intentsQueue chan intents.Intent) {
	f, err := os.OpenFile("client.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)

	messagesQueue := make(chan network.Message, 100)
	mutatorsQueue := make(chan mutators.Mutator, 100)
	uiEvents := make(chan termbox.Event, 100)

	go handleMutators(locker, mutatorsQueue)
	go handleTunnel(locker, tunnel, messagesQueue, mutatorsQueue)
	go handleIntents(locker, tunnel.ID, intentsQueue, messagesQueue)

	go pollTerminalEvents(uiEvents)

	err = termbox.Init()
	if err != nil {
		panic(err)
	}
	defer termbox.Close()
	termbox.SetInputMode(termbox.InputAlt | termbox.InputMouse)

	ticker := time.NewTicker(33 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case ev := <-uiEvents:
			if ev.Ch == 'q' {
				close(messagesQueue)
				close(mutatorsQueue)
				close(uiEvents)
				return
			} else if ev.Key == termbox.KeyArrowUp {
				moveTo(locker, entityID, up, intentsQueue)
			} else if ev.Key == termbox.KeyArrowDown {
				moveTo(locker, entityID, down, intentsQueue)
			} else if ev.Key == termbox.KeyArrowLeft {
				moveTo(locker, entityID, left, intentsQueue)
			} else if ev.Key == termbox.KeyArrowRight {
				moveTo(locker, entityID, right, intentsQueue)
			}

		case _ = <-ticker.C:
			err := termbox.Clear(termbox.ColorWhite, termbox.ColorBlack)
			if err != nil {
				panic(err)
			}

			containers := locker.All()
			for _, container := range containers {
				container.Lock.RLock()
				entity := container.Entity

				char := '@'
				if entity.Spatial.Toggleable {
					char = '+'
					if entity.Spatial.Stackable {
						char = '-'
					}
				} else if !uuid.Equal(entityID, entity.ID) {
					char = '#'
				}

				termbox.SetCell(
					entity.Position.X,
					entity.Position.Y,
					char,
					termbox.ColorWhite,
					termbox.ColorBlack,
				)
				container.Lock.RUnlock()
			}

			c.Render()
			termbox.Flush()
		default:
		}
	}
}

func pollTerminalEvents(queue chan termbox.Event) {
	for {
		queue <- termbox.PollEvent()
	}
}

func handleMutators(locker *entities.Locker, queue chan mutators.Mutator) {
	for mutator := range queue {
		mutator.Mutate(locker)
	}
}

func handleTunnel(
	locker *entities.Locker,
	tunnel network.Tunnel,
	messagesQueue chan network.Message,
	mutatorsQueue chan mutators.Mutator,
) {
	for {
		select {
		case message := <-messagesQueue:
			tunnel.Outgoing <- message
		case message := <-tunnel.Incoming:
			mutator, err := mutators.Unmarshal(message.ContentType, message.Content)
			if err != nil {
				panic(err)
			}

			mutatorsQueue <- mutator
		default:
			// no-op
		}
	}
}

func handleIntents(
	locker *entities.Locker,
	serverID uuid.UUID,
	queue chan intents.Intent,
	messagesQueue chan network.Message,
) {
	for intent := range queue {
		messagesQueue <- network.MakeMessage(
			serverID,
			intent,
		)
	}
}

func moveTo(locker *entities.Locker, sourceID uuid.UUID, dir direction, queue chan intents.Intent) {
	sourceContainer, err := locker.GetByID(sourceID)
	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}
	sourceContainer.Lock.RLock()
	defer sourceContainer.Lock.RUnlock()

	sourceEntity := sourceContainer.Entity

	x := sourceEntity.Position.X
	y := sourceEntity.Position.Y

	switch dir {
	case up:
		y--
	case right:
		x++
	case down:
		y++
	case left:
		x--
	}

	targetPos := components.Position{
		X: x,
		Y: y,
	}

	containers, err := locker.GetByPosition(targetPos)
	if err != nil || len(containers) == 0 {
		queue <- intents.Move{
			SourceID: sourceID,
			Position: targetPos,
		}
		queue <- intents.Perceive{SourceID: sourceID}
		//panic(err)
	} else {

		isPassable := true

		for _, container := range containers {
			container.Lock.RLock()
			targetEntity := container.Entity

			if !targetEntity.Spatial.Stackable {
				isPassable = false
				if !targetEntity.Spatial.Toggleable {
					return
				}

				queue <- intents.OpenSpatial{
					SourceID: sourceID,
					TargetID: targetEntity.ID,
				}
			}

			container.Lock.RUnlock()
		}

		if isPassable {
			queue <- intents.Move{
				SourceID: sourceID,
				Position: targetPos,
			}
			queue <- intents.Perceive{SourceID: sourceID}
		}
	}
}
