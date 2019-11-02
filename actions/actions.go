package actions

import (
	"encoding/json"

	"github.com/clagraff/devoid/components"
	"github.com/clagraff/devoid/entities"

	errs "github.com/go-errors/errors"
	uuid "github.com/satori/go.uuid"
)

type Action interface {
	Execute(*entities.Locker)
}

func Unmarshal(kind string, bytes []byte) (Action, error) {
	var err error
	var action Action

	switch kind {
	case "actions.MoveTo":
		moveToAction := MoveTo{}
		err = json.Unmarshal(bytes, &moveToAction)
		action = moveToAction
	case "actions.MoveFrom":
		moveFromAction := MoveFrom{}
		err = json.Unmarshal(bytes, &moveFromAction)
		action = moveFromAction
	case "actions.SetEntity":
		setEntityAction := SetEntity{}
		err = json.Unmarshal(bytes, &setEntityAction)
		action = setEntityAction
	case "actions.ClearAllEntities":
		mut := ClearAllEntities{}
		err = json.Unmarshal(bytes, &mut)
		action = mut
	case "actions.SetStackability":
		mut := SetStackability{}
		err = json.Unmarshal(bytes, &mut)
		action = mut
	default:
		return nil, errs.Errorf("invalid action kind: %s", kind)
	}

	if err == nil {
		return action, err
	}

	return nil, errs.New(err)
}

type MoveTo struct {
	SourceID uuid.UUID
	Position components.Position
}

func (moveTo MoveTo) Execute(locker *entities.Locker) {
	container, err := locker.GetByID(moveTo.SourceID)
	if err != nil {
		panic(err)
	}
	container.RLock()
	entity := *container.GetEntity()
	container.RUnlock()

	entity.Position.X = moveTo.Position.X
	entity.Position.Y = moveTo.Position.Y

	locker.Set(entity)
}

type MoveFrom struct {
	SourceID uuid.UUID
	Position components.Position
}

func (moveFrom MoveFrom) Execute(locker *entities.Locker) {
	locker.DeleteFromPos(moveFrom.SourceID, moveFrom.Position)
}

type SetEntity struct {
	Entity entities.Entity
}

func (setEntity SetEntity) Execute(locker *entities.Locker) {
	locker.Set(setEntity.Entity)
}

type SetStackability struct {
	Entity       entities.Entity
	Stackability bool
}

func (m SetStackability) Execute(locker *entities.Locker) {
	entity := m.Entity
	entity.Spatial.Stackable = m.Stackability
	locker.Set(entity)
}

type ClearAllEntities struct{}

func (_ ClearAllEntities) Execute(locker *entities.Locker) {
	locker.DeleteAll()
}
