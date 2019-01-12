package mutators

import (
	"encoding/json"

	"github.com/clagraff/devoid/components"
	"github.com/clagraff/devoid/entities"
	"github.com/clagraff/devoid/state"

	errs "github.com/go-errors/errors"
	uuid "github.com/satori/go.uuid"
)

type Mutator interface {
	Mutate(*state.State)
}

func Unmarshal(kind string, bytes []byte) (Mutator, error) {
	var err error
	var mutator Mutator

	switch kind {
	case "mutators.MoveTo":
		moveToMutator := MoveTo{}
		err = json.Unmarshal(bytes, &moveToMutator)
		mutator = moveToMutator
	case "mutators.MoveFrom":
		moveFromMutator := MoveFrom{}
		err = json.Unmarshal(bytes, &moveFromMutator)
		mutator = moveFromMutator
	case "mutators.SetEntity":
		setEntityMutator := SetEntity{}
		err = json.Unmarshal(bytes, &setEntityMutator)
		mutator = setEntityMutator
	default:
		return nil, errs.Errorf("invalid mutator kind: %s", kind)
	}

	if err == nil {
		return mutator, err
	}

	return nil, errs.New(err)
}

type MoveTo struct {
	SourceID uuid.UUID
	Position components.Position
}

func (moveTo MoveTo) Mutate(state *state.State) {
	sourceEntity, unlock, ok := state.ByID(moveTo.SourceID)
	if !ok {
		panic("shit could not unlcok entity")
	}
	defer unlock()

	sourceEntity.Position.X = moveTo.Position.X
	sourceEntity.Position.Y = moveTo.Position.Y

	state.UpsertPosition(sourceEntity)
}

type MoveFrom struct {
	SourceID uuid.UUID
	Position components.Position
}

func (moveFrom MoveFrom) Mutate(state *state.State) {
	state.DeleteIDFromPosition(moveFrom.SourceID, moveFrom.Position)
}

type SetEntity struct {
	Entity entities.Entity
}

func (setEntity SetEntity) Mutate(state *state.State) {
	state.Upsert(&setEntity.Entity)
}
