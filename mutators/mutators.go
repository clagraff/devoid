package mutators

import (
	"encoding/json"

	"github.com/clagraff/devoid/components"
	"github.com/clagraff/devoid/entities"

	errs "github.com/go-errors/errors"
	uuid "github.com/satori/go.uuid"
)

type Mutator interface {
	Mutate(*entities.Locker)
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
	case "mutators.ClearAllEntities":
		mut := ClearAllEntities{}
		err = json.Unmarshal(bytes, &mut)
		mutator = mut
	case "mutators.SetStackability":
		mut := SetStackability{}
		err = json.Unmarshal(bytes, &mut)
		mutator = mut
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

func (moveTo MoveTo) Mutate(locker *entities.Locker) {
	container, err := locker.GetByID(moveTo.SourceID)
	if err != nil {
		panic(err)
	}
	container.GetRWMux().RLock()
	entity := *container.GetEntity()
	container.GetRWMux().RUnlock()

	entity.Position.X = moveTo.Position.X
	entity.Position.Y = moveTo.Position.Y

	locker.Set(entity)
}

type MoveFrom struct {
	SourceID uuid.UUID
	Position components.Position
}

func (moveFrom MoveFrom) Mutate(locker *entities.Locker) {
	locker.DeleteFromPos(moveFrom.SourceID, moveFrom.Position)
}

type SetEntity struct {
	Entity entities.Entity
}

func (setEntity SetEntity) Mutate(locker *entities.Locker) {
	locker.Set(setEntity.Entity)
}

type SetStackability struct {
	Entity       entities.Entity
	Stackability bool
}

func (m SetStackability) Mutate(locker *entities.Locker) {
	entity := m.Entity
	entity.Spatial.Stackable = m.Stackability
	locker.Set(entity)
}

type ClearAllEntities struct{}

func (_ ClearAllEntities) Mutate(locker *entities.Locker) {
	locker.DeleteAll()
}
