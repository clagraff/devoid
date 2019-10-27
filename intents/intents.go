package intents

import (
	"encoding/json"
	"math"

	"github.com/clagraff/devoid/components"
	"github.com/clagraff/devoid/entities"
	"github.com/clagraff/devoid/mutators"
	"github.com/clagraff/devoid/pubsub"

	errs "github.com/go-errors/errors"
	uuid "github.com/satori/go.uuid"
)

type Intent interface {
	Compute(*entities.Locker) ([]mutators.Mutator, []pubsub.Notification)
}

func Unmarshal(kind string, bytes []byte) (Intent, error) {
	var err error
	var intent Intent

	switch kind {
	case "intents.Move":
		moveIntent := Move{}
		err = json.Unmarshal(bytes, &moveIntent)
		intent = moveIntent
	case "intents.Info":
		infoIntent := Info{}
		err = json.Unmarshal(bytes, &infoIntent)
		intent = infoIntent
	case "intents.Perceive":
		perceiveIntent := Perceive{}
		err = json.Unmarshal(bytes, &perceiveIntent)
		intent = perceiveIntent
	case "intents.OpenSpatial":
		openSpatialIntent := OpenSpatial{}
		err = json.Unmarshal(bytes, &openSpatialIntent)
		intent = openSpatialIntent
	case "intents.CloseSpatial":
		closeSpatialIntent := CloseSpatial{}
		err = json.Unmarshal(bytes, &closeSpatialIntent)
		intent = closeSpatialIntent
	default:
		return nil, errs.New("invalid intent kind: " + kind)
	}

	if err == nil {
		return intent, err
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
	sourceContainer.Lock.RLock()
	defer sourceContainer.Lock.RUnlock()

	sourceEntity := sourceContainer.Entity
	xDiff := float64(sourceEntity.Position.X - move.Position.X)
	yDiff := float64(sourceEntity.Position.Y - move.Position.Y)

	if math.Abs(xDiff) > 1 || math.Abs(yDiff) > 1 {
		panic(errs.Errorf("desired Move position is too far away"))
	}

	containersAtPosition, err := locker.GetByPosition(move.Position)
	if err != nil {
		panic("shit went wrong")
	}

	for _, container := range containersAtPosition {
		if container.Entity == sourceContainer.Entity {
			panic("cannot move if already there")
		}
		container.Lock.RLock()
		if !container.Entity.Spatial.Stackable {
			return nil, nil
		}
		container.Lock.RUnlock()
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
	sourceContainer.Lock.RLock()
	defer sourceContainer.Lock.RUnlock()

	sourceEntity := sourceContainer.Entity

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

func (intent Perceive) Compute(locker *entities.Locker) ([]mutators.Mutator, []pubsub.Notification) {
	sourceContainer, err := locker.GetByID(intent.SourceID)
	if err != nil {
		panic("compute perceive went wrong")
	}
	sourceContainer.Lock.RLock()
	sourceEntity := sourceContainer.Entity
	sourcePosition := sourceEntity.Position
	sourceContainer.Lock.RUnlock()

	visibility := 5
	minX := sourcePosition.X - visibility
	maxX := sourcePosition.X + visibility

	minY := sourcePosition.Y - visibility
	maxY := sourcePosition.Y + visibility

	muts := make([]mutators.Mutator, 0)

	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			containers, err := locker.GetByPosition(components.Position{x, y})
			if err != nil {
				panic("oh shit doesnt work")
			}

			for _, container := range containers {
				if container.Entity == sourceEntity {
					continue
				}
				container.Lock.RLock()

				muts = append(muts, mutators.SetEntity{Entity: *container.Entity})
				container.Lock.RUnlock()
			}
		}
	}

	notifications := []pubsub.Notification{
		pubsub.Notification{
			Type:     intent.SourceID,
			Mutators: []mutators.Mutator{mutators.ClearAllEntities{}},
		},
		pubsub.Notification{
			Type:     intent.SourceID,
			Mutators: muts,
		},
	}

	return nil, notifications
}

type OpenSpatial struct {
	SourceID uuid.UUID
	TargetID uuid.UUID
}

func (intent OpenSpatial) Compute(locker *entities.Locker) ([]mutators.Mutator, []pubsub.Notification) {
	if uuid.Equal(intent.SourceID, intent.TargetID) {
		panic("cannot open yourself I think")
	}

	sourceContainer, err := locker.GetByID(intent.SourceID)
	if err != nil {
		panic("compute info went wrong")
	}
	sourceContainer.Lock.RLock()
	defer sourceContainer.Lock.RUnlock()

	targetContainer, err := locker.GetByID(intent.TargetID)
	if err != nil {
		panic("compute OpenSpatial went wrong")
	}
	targetContainer.Lock.RLock()
	defer targetContainer.Lock.RUnlock()

	targetEntity := targetContainer.Entity
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
			Type:     intent.TargetID,
			Mutators: []mutators.Mutator{mutate},
		},
		pubsub.Notification{
			Type:     intent.SourceID,
			Mutators: []mutators.Mutator{mutate},
		},
	}

	return []mutators.Mutator{mutate}, notifications
}

type CloseSpatial struct {
	SourceID uuid.UUID
	TargetID uuid.UUID
}

func (intent CloseSpatial) Compute(locker *entities.Locker) ([]mutators.Mutator, []pubsub.Notification) {
	sourceContainer, err := locker.GetByID(intent.SourceID)
	if err != nil {
		panic("compute info went wrong")
	}
	sourceContainer.Lock.RLock()
	defer sourceContainer.Lock.RUnlock()

	sourceEntity := sourceContainer.Entity

	targetContainer, err := locker.GetByID(intent.TargetID)
	if err != nil {
		panic("compute info went wrong")
	}
	targetContainer.Lock.RLock()
	defer targetContainer.Lock.RUnlock()

	targetEntity := targetContainer.Entity

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
			Type:     intent.TargetID,
			Mutators: []mutators.Mutator{mutate},
		},
	}

	return nil, notifications
}
