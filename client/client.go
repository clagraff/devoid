package client

import (
	"fmt"
	"time"

	"github.com/clagraff/devoid/components"
	"github.com/clagraff/devoid/intents"
	"github.com/clagraff/devoid/mutators"
	"github.com/clagraff/devoid/network"
	"github.com/clagraff/devoid/state"

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

func Serve(entityID uuid.UUID, state *state.State, tunnel network.Tunnel, intentsQueue chan intents.Intent) {

	messagesQueue := make(chan network.Message, 100)
	mutatorsQueue := make(chan mutators.Mutator, 100)
	uiEvents := make(chan termbox.Event, 100)

	go handleMutators(state, mutatorsQueue)
	go handleTunnel(state, tunnel, messagesQueue, mutatorsQueue)
	go handleIntents(state, tunnel.ID, intentsQueue, messagesQueue)

	go pollTerminalEvents(uiEvents)

	err := termbox.Init()
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
			} else if ev.Ch == 'i' {
				e, u, _ := state.ByPosition(components.Position{X: 3, Y: 7})
				fmt.Println(e)
				u()
			} else if ev.Key == termbox.KeyArrowUp {
				moveTo(state, entityID, up, intentsQueue)
			} else if ev.Key == termbox.KeyArrowDown {
				moveTo(state, entityID, down, intentsQueue)
			} else if ev.Key == termbox.KeyArrowLeft {
				moveTo(state, entityID, left, intentsQueue)
			} else if ev.Key == termbox.KeyArrowRight {
				moveTo(state, entityID, right, intentsQueue)
			} else if ev.Ch == 'f' {
				entity, unlock, ok := state.ByID(entityID)
				if ok {
					intentsQueue <- intents.Move{
						SourceID: entityID,
						Position: components.Position{
							X: entity.Position.X + 1,
							Y: entity.Position.Y + 1,
						},
					}
					intentsQueue <- intents.Perceive{SourceID: entityID}

					unlock()
				}
			}

		case _ = <-ticker.C:
			err := termbox.Clear(termbox.ColorWhite, termbox.ColorBlack)
			if err != nil {
				panic(err)
			}

			ids := state.ListIDs()
			for _, id := range ids {
				entity, unlock, ok := state.ByID(id)
				if !ok {

					panic("failed to render entity")
				}

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
				unlock()
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

func handleMutators(state *state.State, queue chan mutators.Mutator) {
	for mutator := range queue {
		handleMutator(state, mutator)
	}
}

func handleTunnel(
	state *state.State,
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

func handleMutator(state *state.State, mutator mutators.Mutator) {
	if mutator != nil {
		mutator.Mutate(state)
	} else {
		// panic?
	}
}

func handleIntents(
	state *state.State,
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

func moveTo(state *state.State, sourceID uuid.UUID, dir direction, queue chan intents.Intent) {
	sourceEntity, sourceUnlock, ok := state.ByID(sourceID)
	defer sourceUnlock()

	if !ok {
		return
	}

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

	targetEntities, targetUnlock, ok := state.ByPosition(targetPos)
	defer targetUnlock()

	if !ok {
		queue <- intents.Move{
			SourceID: sourceID,
			Position: targetPos,
		}
		queue <- intents.Perceive{SourceID: sourceID}
	} else {
		isPassable := true

		for _, targetEntity := range targetEntities {
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
