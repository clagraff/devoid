package client

import (
	"fmt"
	"os"
	"time"

	"github.com/clagraff/devoid/actions"
	"github.com/clagraff/devoid/commands"
	"github.com/clagraff/devoid/components"
	"github.com/clagraff/devoid/entities"
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

func Serve(entityID uuid.UUID, locker *entities.Locker, tunnel network.Tunnel, commandsQueue chan commands.Command) {
	messagesQueue := make(chan network.Message, 100)
	actionsQueue := make(chan actions.Action, 100)
	uiEvents := make(chan termbox.Event, 100)

	go handleActions(locker, actionsQueue)
	go handleTunnel(locker, tunnel, messagesQueue, actionsQueue)
	go handleCommands(tunnel.ID, commandsQueue, messagesQueue)

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
				close(actionsQueue)
				close(uiEvents)
				return
			} else if ev.Key == termbox.KeyArrowUp {
				moveTo(locker, entityID, up, commandsQueue)
			} else if ev.Key == termbox.KeyArrowDown {
				moveTo(locker, entityID, down, commandsQueue)
			} else if ev.Key == termbox.KeyArrowLeft {
				moveTo(locker, entityID, left, commandsQueue)
			} else if ev.Key == termbox.KeyArrowRight {
				moveTo(locker, entityID, right, commandsQueue)
			}

		case _ = <-ticker.C:
			err := termbox.Clear(termbox.ColorWhite, termbox.ColorBlack)
			if err != nil {
				panic(err)
			}

			containers := locker.All()
			for _, container := range containers {
				container.RLock()
				entity := container.GetEntity()

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
				container.RUnlock()
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

func handleActions(locker *entities.Locker, queue chan actions.Action) {
	for action := range queue {
		action.Mutate(locker)
	}
}

func handleTunnel(
	locker *entities.Locker,
	tunnel network.Tunnel,
	messagesQueue chan network.Message,
	actionsQueue chan actions.Action,
) {
	for {
		select {
		case message := <-messagesQueue:
			tunnel.Outgoing <- message
		case message := <-tunnel.Incoming:
			action, err := actions.Unmarshal(message.ContentType, message.Content)
			if err != nil {
				panic(err)
			}

			actionsQueue <- action
		default:
			// no-op
		}
	}
}

func handleCommands(
	serverID uuid.UUID,
	queue chan commands.Command,
	messagesQueue chan network.Message,
) {
	for command := range queue {
		messagesQueue <- network.MakeMessage(
			serverID,
			command,
		)
	}
}

func moveTo(locker *entities.Locker, sourceID uuid.UUID, dir direction, queue chan commands.Command) {
	sourceContainer, err := locker.GetByID(sourceID)
	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}
	sourceContainer.RLock()
	defer sourceContainer.RUnlock()

	sourceEntity := sourceContainer.GetEntity()

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
	if len(containers) == 0 {
		queue <- commands.Move{
			SourceID: sourceID,
			Position: targetPos,
		}
		queue <- commands.Perceive{SourceID: sourceID}
	} else {

		isPassable := true

		for _, container := range containers {
			container.RLock()
			targetEntity := container.GetEntity()

			if !targetEntity.Spatial.Stackable {
				isPassable = false
				if !targetEntity.Spatial.Toggleable {
					return
				}

				queue <- commands.OpenSpatial{
					SourceID: sourceID,
					TargetID: targetEntity.ID,
				}
			}

			container.RUnlock()
		}

		if isPassable {
			queue <- commands.Move{
				SourceID: sourceID,
				Position: targetPos,
			}
			queue <- commands.Perceive{SourceID: sourceID}
		}
	}
}
