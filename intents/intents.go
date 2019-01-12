package intents

import (
	"encoding/json"
	"math"

	"bitbucket.org/clagraff/yawning/components"
	"bitbucket.org/clagraff/yawning/entities"
	"bitbucket.org/clagraff/yawning/mutators"
	"bitbucket.org/clagraff/yawning/pubsub"
	"bitbucket.org/clagraff/yawning/state"

	errs "github.com/go-errors/errors"
	uuid "github.com/satori/go.uuid"
)

type Intent interface {
	Validate(*state.State) []pubsub.Notification

	Compute(*state.State) []pubsub.Notification
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

func (move Move) Validate(state *state.State) []pubsub.Notification {
	sourceEntity, unlock, ok := state.ByID(move.SourceID)
	if !ok {
		panic("could not locate entity")
	}
	defer unlock()

	xDiff := float64(sourceEntity.Position.X - move.Position.X)
	yDiff := float64(sourceEntity.Position.Y - move.Position.Y)

	if math.Abs(xDiff) > 1 || math.Abs(yDiff) > 1 {
		panic(errs.Errorf("desired Move position is too far away"))
	}

	entitiesAtPosition, unlockPos, ok := state.ByPosition(move.Position)
	if !ok {
		panic("shit went wrong")
	}
	defer unlockPos()

	for _, entity := range entitiesAtPosition {
		if entity.Spatial.OccupiesPosition {
			panic(errs.Errorf("cannot Move to occupied Position"))
		}
	}

	return nil
}

func (move Move) Compute(state *state.State) []pubsub.Notification {
	sourceEntity, unlock, ok := state.ByID(move.SourceID)
	if !ok {
		panic("shit didnt work bro")
	}
	defer unlock()

	moveTo := mutators.MoveTo{
		SourceID: move.SourceID,
		Position: move.Position,
	}

	moveFrom := mutators.MoveFrom{
		SourceID: move.SourceID,
		Position: sourceEntity.Position,
	}

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
			Type:     nil,
			Mutators: []mutators.Mutator{moveTo, moveFrom},
		},
		pubsub.Notification{
			Type:     sourceEntity.ID,
			Mutators: []mutators.Mutator{moveTo, moveFrom},
		},
	}

	return notifications
}

type Info struct {
	SourceID uuid.UUID
}

func (info Info) Validate(_ *state.State) []pubsub.Notification { return nil }

func (info Info) Compute(state *state.State) []pubsub.Notification {
	sourceEntity, unlock, ok := state.ByID(info.SourceID)
	if !ok {
		panic("compute info went wrong")
	}
	defer unlock()

	inform := mutators.SetEntity{
		Entity: *sourceEntity,
	}

	notifications := []pubsub.Notification{
		pubsub.Notification{
			Type:     info.SourceID,
			Mutators: []mutators.Mutator{inform},
		},
	}

	return notifications
}

type Perceive struct {
	SourceID uuid.UUID
}

func (intent Perceive) Validate(_ *state.State) []pubsub.Notification { return nil }

func (intent Perceive) Compute(state *state.State) []pubsub.Notification {
	sourceEntity, unlock, ok := state.ByID(intent.SourceID)
	if !ok {
		panic("compute perceive went wrong")
	}
	sourcePosition := sourceEntity.Position
	unlock()

	visibility := 5
	minX := sourcePosition.X - visibility
	maxX := sourcePosition.X + visibility

	minY := sourcePosition.Y - visibility
	maxY := sourcePosition.Y + visibility

	ents := make([]*entities.Entity, 0)
	muts := make([]mutators.Mutator, 0)

	for x := minX; x <= maxX; x++ {
		for y := minY; y <= maxY; y++ {
			entitiesAtPosition, unlock, ok := state.ByPosition(components.Position{x, y})
			if ok {
				ents = append(ents, entitiesAtPosition...)

				for _, e := range entitiesAtPosition {
					muts = append(muts, mutators.SetEntity{Entity: *e})
				}

				unlock()
			}
		}
	}

	notifications := []pubsub.Notification{
		pubsub.Notification{
			Type:     intent.SourceID,
			Mutators: muts,
		},
	}

	return notifications
}
