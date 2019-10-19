package entities

import (
	"encoding/json"
	"io/ioutil"
	"sync"

	"github.com/clagraff/devoid/components"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

// multiLock satisfies the sync.Locker interface, for the purpose of
// locking and unlocking a list of sync.Locker values sequentially.
type multiLock struct {
	locks []sync.Locker
}

// Lock will sequentially lock all sync.Locker values currently contained
// in the multiLock.
func (ml multiLock) Lock() {
	for i, l := range ml.locks {
		l.Lock()
	}
}

// Unlock will sequentially unlock all sync.Locker values currently contained
// in the multiLock.
func (ml multiLock) Unlock() {
	for _, l := range ml.locks {
		l.Unlock()
	}
}

// entityContainer serves to maintain both a single *Entity and a RWMutex
// for access.
type entityContainer struct {
	Lock   *sync.RWMutex
	Entity *Entity
}

func makeEntityContainer() entityContainer {
	return entityContainer{
		Lock:   new(sync.RWMutex),
		Entity: new(Entity),
	}
}

// idContainer is a concurrent-use map of UUID ID to entityContainer.
type idContainer struct {
	internal *sync.Map
}

// Delete the entityContainer at the provided ID if present, or do nothing.
func (c idContainer) Delete(id uuid.UUID) {
	c.internal.Delete(id)
}

// Load returns the entityContainer for the provided id, with a boolean
// indicating success.
func (c idContainer) Load(id uuid.UUID) (entityContainer, bool) {
	var container entityContainer

	value, ok := c.internal.Load(id)
	if ok {
		container = value.(entityContainer)
	}

	return container, ok
}

// Store will save the provided entityContainer at the given id; this will
// override any previous value stored at that id.
func (c idContainer) Store(id uuid.UUID, container entityContainer) {
	c.internal.Store(id, container)
}

// All returns a list of entityContainer stored in this map.
// Modifications to the map while this function runs may impact the results.
func (c idContainer) All() []entityContainer {
	allContainers := make([]entityContainer, 0)

	ranger := func(key, value interface{}) bool {
		if ec, ok := value.(entityContainer); ok {
			allContainers = append(allContainers, ec)
		}
		return true
	}

	c.internal.Range(ranger)
	return allContainers
}

// makeIDContainer returns an instantiated idContainer.
func makeIDContainer() idContainer {
	return idContainer{
		internal: new(sync.Map),
	}
}

// posContainer is a concurrent-use map of components.Position to idContainer.
type posContainer struct {
	internal *sync.Map
}

// Delete the idContainer at the provided position if present, or do nothing.
func (c posContainer) Delete(pos components.Position) {
	c.internal.Delete(pos)
}

// Load returns the idContainer for the provided position, with a boolean
// indicating success.
func (c posContainer) Load(pos components.Position) (idContainer, bool) {
	var container idContainer

	value, ok := c.internal.Load(pos)
	if ok {
		container = value.(idContainer)
	}

	return container, ok
}

// Store will save the provided idContainer at the given position; this will
// override any previous value stored at that position.
func (c posContainer) Store(pos components.Position, container idContainer) {
	c.internal.Store(pos, container)
}

// makePosContainer returns an instantiated posContainer.
func makePosContainer() posContainer {
	return posContainer{
		internal: new(sync.Map),
	}
}

type Locker struct {
	byID  idContainer
	byPos posContainer
}

func (l Locker) GetByID(id uuid.UUID) (Entity, sync.Locker, error) {
	container, ok := l.byID.Load(id)
	if !ok {
		return Entity{}, nil, errors.Errorf("no entity with id %s", id)
	}

	return *container.Entity, container.Lock.RLocker(), nil
}

func (l Locker) GetByPosition(pos components.Position) ([]Entity, sync.Locker, error) {
	entitiesByID, ok := l.byPos.Load(pos)
	if !ok {
		return nil, nil, errors.Errorf("no position for %+v", pos)
	}

	entities := make([]Entity, 0)
	rLocks := make([]sync.Locker, 0)

	for _, container := range entitiesByID.All() {
		entities = append(entities, *container.Entity)
		rLocks = append(rLocks, container.Lock.RLocker())
	}

	allLocks := multiLock{rLocks}

	return entities, allLocks, nil
}

func (l Locker) Set(entity Entity) error {
	// Grab the entity and lock it.
	id := entity.ID

	entityContainer, ok := l.byID.Load(id)
	if !ok {
		return errors.Errorf("no entity with id %s", id)
	}

	entityContainer.Lock.Lock()
	defer entityContainer.Lock.Unlock()

	// Grab old version. Check if position differs.

	oldEntity := entityContainer.Entity
	oldPos := oldEntity.Position

	// If position changed, remove from old pos.
	if (oldPos.X != entity.Position.X) || (oldPos.Y != entity.Position.Y) {
		ids, ok := l.byPos.Load(oldPos)
		if !ok {
			return errors.Errorf("no position %+v", oldPos)
		}
		ids.Delete(id)
	}

	// Update entity contents in container. Update in ID store.
	*entityContainer.Entity = entity
	l.byID.Store(id, entityContainer)

	// Update in position store.
	newPos := entity.Position

	ids, ok := l.byPos.Load(newPos)
	if !ok {
		l.byPos.Store(newPos, makeIDContainer())
	}

	ids.Store(id, entityContainer)

	l.byPos.Store(newPos, ids)
	return nil
}

func (l *Locker) FromJSONFile(path string) error {
	// try to read the file
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Wrapf(err, "could not find json file %s", path)
	}

	// try to unmarshal into entity list
	allEntities := make([]Entity, 0)
	err = json.Unmarshal(bytes, allEntities)
	if err != nil {
		return errors.Wrapf(err, "not a valid json file %s", path)
	}

	for _, entity := range allEntities {
		container := makeEntityContainer()
		*container.Entity = entity

		l.byID.Store(entity.ID, container)

		pos := entity.Position

		if ids, ok := l.byPos.Load(pos); ok {
			ids.Store(entity.ID, container)
		} else {
			ids = makeIDContainer()
			ids.Store(entity.ID, container)
			l.byPos.Store(pos, ids)
		}
	}

	return nil
}

func makeLocker() Locker {
	return Locker{
		byID:  makeIDContainer(),
		byPos: makePosContainer(),
	}
}
