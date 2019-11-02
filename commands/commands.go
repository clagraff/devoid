package commands

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/clagraff/devoid/components"
	"github.com/clagraff/devoid/entities"
	"github.com/clagraff/devoid/mutators"
	"github.com/clagraff/devoid/pubsub"

	errs "github.com/go-errors/errors"
	uuid "github.com/satori/go.uuid"
)

type Command interface {
	Compute(*entities.Locker) ([]mutators.Mutator, []pubsub.Notification)
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

func (move Move) Compute(locker *entities.Locker) ([]mutators.Mutator, []pubsub.Notification) {
	sourceContainer, err := locker.GetByID(move.SourceID)
	if err != nil {
		panic("could not locate entity")
	}
	sourceContainer.RLock()
	defer sourceContainer.RUnlock()

	sourceEntity := sourceContainer.GetEntity()
	xDiff := float64(sourceEntity.Position.X - move.Position.X)
	yDiff := float64(sourceEntity.Position.Y - move.Position.Y)

	if math.Abs(xDiff) > 1 || math.Abs(yDiff) > 1 {
		panic(errs.Errorf("desired Move position is too far away"))
	}

	containersAtPosition, _ := locker.GetByPosition(move.Position)

	for _, container := range containersAtPosition {
		if container.GetEntity() == sourceContainer.GetEntity() {
			panic("cannot move to where you are already at")
		}
		container.RLock()
		if !container.GetEntity().Spatial.Stackable {
			return nil, nil
		}
		container.RUnlock()
	}

	moveTo := mutators.MoveTo{
		SourceID: move.SourceID,
		Position: move.Position,
	}

	moveFrom := mutators.MoveFrom{
		SourceID: move.SourceID,
		Position: sourceEntity.Position,
	}

	serverMutations := []mutators.Mutator{moveTo, moveFrom}
	notifications := []pubsub.Notification{
		pubsub.Notification{
			Type:     move.Position,
			Mutators: []mutators.Mutator{moveTo},
		},
		pubsub.Notification{
			Type:     sourceEntity.Position,
			Mutators: []mutators.Mutator{moveFrom},
		},
		pubsub.Notification{
			Type:     sourceEntity.ID,
			Mutators: []mutators.Mutator{moveTo, moveFrom},
		},
	}

	return serverMutations, notifications
}

type Info struct {
	SourceID uuid.UUID
}

func (info Info) Compute(locker *entities.Locker) ([]mutators.Mutator, []pubsub.Notification) {
	sourceContainer, err := locker.GetByID(info.SourceID)
	if err != nil {
		panic("compute info went wrong")
	}
	sourceContainer.RLock()
	defer sourceContainer.RUnlock()

	sourceEntity := sourceContainer.GetEntity()

	inform := mutators.SetEntity{
		Entity: *sourceEntity,
	}

	notifications := []pubsub.Notification{
		pubsub.Notification{
			Type:     info.SourceID,
			Mutators: []mutators.Mutator{inform},
		},
	}

	return nil, notifications
}

type Perceive struct {
	SourceID uuid.UUID
}

func (command Perceive) Compute(locker *entities.Locker) ([]mutators.Mutator, []pubsub.Notification) {
	sourceContainer, err := locker.GetByID(command.SourceID)
	if err != nil {
		fmt.Printf("%+v\n", err)
		os.Exit(1)
	}
	sourceContainer.RLock()
	sourceEntity := sourceContainer.GetEntity()
	sourcePosition := sourceEntity.Position
	sourceContainer.RUnlock()

	visibility := 5
	minX := sourcePosition.X - visibility
	maxX := sourcePosition.X + visibility

	minY := sourcePosition.Y - visibility
	maxY := sourcePosition.Y + visibility

	muts := make([]mutators.Mutator, 0)

	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			containers, _ := locker.GetByPosition(components.Position{x, y})

			for _, container := range containers {
				container.RLock()
				muts = append(
					muts,
					mutators.SetEntity{Entity: *container.GetEntity()},
				)
				container.RUnlock()
			}
		}
	}

	notifications := []pubsub.Notification{
		pubsub.Notification{
			Type:     command.SourceID,
			Mutators: []mutators.Mutator{mutators.ClearAllEntities{}},
		},
		pubsub.Notification{
			Type:     command.SourceID,
			Mutators: muts,
		},
	}

	return nil, notifications
}

type OpenSpatial struct {
	SourceID uuid.UUID
	TargetID uuid.UUID
}

func (command OpenSpatial) Compute(locker *entities.Locker) ([]mutators.Mutator, []pubsub.Notification) {
	if uuid.Equal(command.SourceID, command.TargetID) {
		panic("cannot open yourself I think")
	}

	sourceContainer, err := locker.GetByID(command.SourceID)
	if err != nil {
		panic("compute info went wrong")
	}
	sourceContainer.RLock()
	defer sourceContainer.RUnlock()

	targetContainer, err := locker.GetByID(command.TargetID)
	if err != nil {
		panic("compute OpenSpatial went wrong")
	}
	targetContainer.RLock()
	defer targetContainer.RUnlock()

	targetEntity := targetContainer.GetEntity()
	// If target is not toggleable, do nothing.
	if !targetEntity.Spatial.Toggleable {
		return nil, nil
	}

	// If target is already passable, do nothing.
	if targetEntity.Spatial.Stackable {
		return nil, nil
	}

	mutate := mutators.SetStackability{
		Entity:       *targetEntity,
		Stackability: true,
	}

	notifications := []pubsub.Notification{
		pubsub.Notification{
			Type:     command.TargetID,
			Mutators: []mutators.Mutator{mutate},
		},
		pubsub.Notification{
			Type:     command.SourceID,
			Mutators: []mutators.Mutator{mutate},
		},
	}

	return []mutators.Mutator{mutate}, notifications
}

type CloseSpatial struct {
	SourceID uuid.UUID
	TargetID uuid.UUID
}

func (command CloseSpatial) Compute(locker *entities.Locker) ([]mutators.Mutator, []pubsub.Notification) {
	sourceContainer, err := locker.GetByID(command.SourceID)
	if err != nil {
		panic("compute info went wrong")
	}
	sourceContainer.RLock()
	defer sourceContainer.RUnlock()

	sourceEntity := sourceContainer.GetEntity()

	targetContainer, err := locker.GetByID(command.TargetID)
	if err != nil {
		panic("compute info went wrong")
	}
	targetContainer.RLock()
	defer targetContainer.RUnlock()

	targetEntity := targetContainer.GetEntity()

	// If target is not toggleable, do nothing.
	if !targetEntity.Spatial.Toggleable {
		return nil, nil
	}

	// If target is already not passable, do nothing.
	if !targetEntity.Spatial.Stackable {
		return nil, nil
	}

	mutate := mutators.SetStackability{
		Entity:       *sourceEntity,
		Stackability: false,
	}

	notifications := []pubsub.Notification{
		pubsub.Notification{
			Type:     command.TargetID,
			Mutators: []mutators.Mutator{mutate},
		},
	}

	return nil, notifications
}
