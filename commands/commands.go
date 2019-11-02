package commands

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/clagraff/devoid/actions"
	"github.com/clagraff/devoid/components"
	"github.com/clagraff/devoid/entities"
	"github.com/clagraff/devoid/pubsub"

	errs "github.com/go-errors/errors"
	uuid "github.com/satori/go.uuid"
)

type Command interface {
	Compute(*entities.Locker) ([]actions.Action, []pubsub.Notification)
}

func Unmarshal(kind string, bytes []byte) (Command, error) {
	var err error
	var command Command

	switch kind {
	case "commands.Move":
		moveCommand := Move{}
		err = json.Unmarshal(bytes, &moveCommand)
		command = moveCommand
	case "commands.Info":
		infoCommand := Info{}
		err = json.Unmarshal(bytes, &infoCommand)
		command = infoCommand
	case "commands.Perceive":
		perceiveCommand := Perceive{}
		err = json.Unmarshal(bytes, &perceiveCommand)
		command = perceiveCommand
	case "commands.OpenSpatial":
		openSpatialCommand := OpenSpatial{}
		err = json.Unmarshal(bytes, &openSpatialCommand)
		command = openSpatialCommand
	case "commands.CloseSpatial":
		closeSpatialCommand := CloseSpatial{}
		err = json.Unmarshal(bytes, &closeSpatialCommand)
		command = closeSpatialCommand
	default:
		return nil, errs.New("invalid command kind: " + kind)
	}

	if err == nil {
		return command, err
	}

	return nil, errs.New(err)
}

type Move struct {
	SourceID uuid.UUID
	Position components.Position
}

func (move Move) Compute(locker *entities.Locker) ([]actions.Action, []pubsub.Notification) {
	sourceEntity, err := locker.GetByID(move.SourceID)
	if err != nil {
		panic("could not locate entity")
	}

	xDiff := float64(sourceEntity.Position.X - move.Position.X)
	yDiff := float64(sourceEntity.Position.Y - move.Position.Y)

	if math.Abs(xDiff) > 1 || math.Abs(yDiff) > 1 {
		panic(errs.Errorf("desired Move position is too far away"))
	}

	entitiesAtPosition, _ := locker.GetByPosition(move.Position)

	for _, entity := range entitiesAtPosition {
		if entity.ID == sourceEntity.ID {
			panic("cannot move to where you are already at")
		}
		if !entity.Spatial.Stackable {
			return nil, nil
		}
	}

	moveTo := actions.MoveTo{
		SourceID: move.SourceID,
		Position: move.Position,
	}

	moveFrom := actions.MoveFrom{
		SourceID: move.SourceID,
		Position: sourceEntity.Position,
	}

	serverMutations := []actions.Action{moveTo, moveFrom}
	notifications := []pubsub.Notification{
		pubsub.Notification{
			Type:    move.Position,
			Actions: []actions.Action{moveTo},
		},
		pubsub.Notification{
			Type:    sourceEntity.Position,
			Actions: []actions.Action{moveFrom},
		},
		pubsub.Notification{
			Type:    sourceEntity.ID,
			Actions: []actions.Action{moveTo, moveFrom},
		},
	}

	return serverMutations, notifications
}

type Info struct {
	SourceID uuid.UUID
}

func (info Info) Compute(locker *entities.Locker) ([]actions.Action, []pubsub.Notification) {
	sourceEntity, err := locker.GetByID(info.SourceID)
	if err != nil {
		panic("compute info went wrong")
	}

	inform := actions.SetEntity{
		Entity: sourceEntity,
	}

	notifications := []pubsub.Notification{
		pubsub.Notification{
			Type:    info.SourceID,
			Actions: []actions.Action{inform},
		},
	}

	return nil, notifications
}

type Perceive struct {
	SourceID uuid.UUID
}

func (command Perceive) Compute(locker *entities.Locker) ([]actions.Action, []pubsub.Notification) {
	sourceEntity, err := locker.GetByID(command.SourceID)
	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}
	sourcePosition := sourceEntity.Position

	visibility := 5
	minX := sourcePosition.X - visibility
	maxX := sourcePosition.X + visibility

	minY := sourcePosition.Y - visibility
	maxY := sourcePosition.Y + visibility

	muts := make([]actions.Action, 0)

	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			entitiesAtPosition, _ := locker.GetByPosition(components.Position{x, y})

			for _, e := range entitiesAtPosition {
				muts = append(
					muts,
					actions.SetEntity{Entity: e},
				)
			}
		}
	}

	notifications := []pubsub.Notification{
		pubsub.Notification{
			Type:    command.SourceID,
			Actions: []actions.Action{actions.ClearAllEntities{}},
		},
		pubsub.Notification{
			Type:    command.SourceID,
			Actions: muts,
		},
	}

	return nil, notifications
}

type OpenSpatial struct {
	SourceID uuid.UUID
	TargetID uuid.UUID
}

func (command OpenSpatial) Compute(locker *entities.Locker) ([]actions.Action, []pubsub.Notification) {
	if uuid.Equal(command.SourceID, command.TargetID) {
		panic("cannot open yourself I think")
	}

	_, err := locker.GetByID(command.SourceID)
	if err != nil {
		panic("compute info went wrong")
	}

	targetEntity, err := locker.GetByID(command.TargetID)
	if err != nil {
		panic("compute OpenSpatial went wrong")
	}

	// If target is not toggleable, do nothing.
	if !targetEntity.Spatial.Toggleable {
		return nil, nil
	}

	// If target is already passable, do nothing.
	if targetEntity.Spatial.Stackable {
		return nil, nil
	}

	mutate := actions.SetStackability{
		Entity:       targetEntity,
		Stackability: true,
	}

	notifications := []pubsub.Notification{
		pubsub.Notification{
			Type:    command.TargetID,
			Actions: []actions.Action{mutate},
		},
		pubsub.Notification{
			Type:    command.SourceID,
			Actions: []actions.Action{mutate},
		},
	}

	return []actions.Action{mutate}, notifications
}

type CloseSpatial struct {
	SourceID uuid.UUID
	TargetID uuid.UUID
}

func (command CloseSpatial) Compute(locker *entities.Locker) ([]actions.Action, []pubsub.Notification) {
	sourceEntity, err := locker.GetByID(command.SourceID)
	if err != nil {
		panic("compute info went wrong")
	}

	targetEntity, err := locker.GetByID(command.TargetID)
	if err != nil {
		panic("compute info went wrong")
	}

	// If target is not toggleable, do nothing.
	if !targetEntity.Spatial.Toggleable {
		return nil, nil
	}

	// If target is already not passable, do nothing.
	if !targetEntity.Spatial.Stackable {
		return nil, nil
	}

	mutate := actions.SetStackability{
		Entity:       sourceEntity,
		Stackability: false,
	}

	notifications := []pubsub.Notification{
		pubsub.Notification{
			Type:    command.TargetID,
			Actions: []actions.Action{mutate},
		},
	}

	return nil, notifications
}
