package entities

import (
	"encoding/json"
	"io/ioutil"
	"log"
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
	for _, l := range ml.locks {
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

// Container serves to maintain both a single *Entity and a RWMutex
// for access.
type Container struct {
	Lock   *sync.RWMutex
	Entity *Entity
}

func makeContainer() Container {
	return Container{
		Lock:   new(sync.RWMutex),
		Entity: new(Entity),
	}
}

// idContainer is a concurrent-use map of UUID ID to Container.
type idContainer struct {
	internal *sync.Map
}

// Delete the Container at the provided ID if present, or do nothing.
func (c idContainer) Delete(id uuid.UUID) {
	c.internal.Delete(id)
}

// Load returns the Container for the provided id, with a boolean
// indicating success.
func (c idContainer) Load(id uuid.UUID) (Container, bool) {
	var container Container

	value, ok := c.internal.Load(id)
	if ok {
		container = value.(Container)
	}

	return container, ok
}

// Store will save the provided Container at the given id; this will
// override any previous value stored at that id.
func (c idContainer) Store(id uuid.UUID, container Container) {
	c.internal.Store(id, container)
}

// All returns a list of Container stored in this map.
// Modifications to the map while this function runs may impact the results.
func (c idContainer) All() []Container {
	allContainers := make([]Container, 0)

	ranger := func(key, value interface{}) bool {
		if ec, ok := value.(Container); ok {
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
	byID  *idContainer
	byPos *posContainer
}

func (l Locker) All() []Container {
	return l.byID.All()
}

func (l Locker) GetByID(id uuid.UUID) (Container, error) {
	container, ok := l.byID.Load(id)
	if !ok {
		return Container{}, errors.Errorf("no entity with id %s", id)
	}

	return container, nil
}

// TODO: Return list of sync.Locker
func (l Locker) GetByPosition(pos components.Position) ([]Container, error) {
	entitiesAtPosition, ok := l.byPos.Load(pos)
	if !ok {
		return nil, errors.Errorf("no position for %s", pos)
	}

	return entitiesAtPosition.All(), nil
}

func (l *Locker) Set(entity Entity) error {
	// Grab the entity and lock it.
	id := entity.ID
	log.Println("stackability", entity.Spatial.Stackable)

	container, ok := l.byID.Load(id)
	if !ok {
		container = makeContainer()
		container.Entity = &entity
	}

	container.Lock.Lock()
	defer container.Lock.Unlock()

	// Grab old version. Check if position differs.

	oldEntity := container.Entity
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
	*container.Entity = entity
	l.byID.Store(id, container)

	// Update in position store.
	newPos := entity.Position

	ids, ok := l.byPos.Load(newPos)
	if !ok {
		ids = makeIDContainer()
	}

	ids.Store(id, container)

	l.byPos.Store(newPos, ids)
	return nil
}

func (l *Locker) Delete(id uuid.UUID) error {
	container, ok := l.byID.Load(id)
	if !ok {
		return errors.Errorf("no entity with id %s", id)
	}

	container.Lock.Lock()
	defer container.Lock.Unlock()

	position := container.Entity.Position

	entitiesAtPosition, ok := l.byPos.Load(position)
	if ok {
		entitiesAtPosition.Delete(id)
		l.byPos.Store(position, entitiesAtPosition)
	}

	l.byID.Delete(id)

	return nil
}

func (l *Locker) DeleteFromPos(id uuid.UUID, pos components.Position) error {
	entitiesAtPosition, ok := l.byPos.Load(pos)
	if ok {
		entitiesAtPosition.Delete(id)
		l.byPos.Store(pos, entitiesAtPosition)
	}

	return nil
}

func (l *Locker) DeleteAll() {
	containers := l.byID.All()
	for _, container := range containers {
		l.Delete(container.Entity.ID)
	}
}

func (l *Locker) FromJSONFile(path string) error {
	// try to read the file
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Wrapf(err, "could not find json file %s", path)
	}

	// try to unmarshal into entity list
	allEntities := make([]Entity, 0)
	err = json.Unmarshal(bytes, &allEntities)
	if err != nil {
		return errors.Wrapf(err, "not a valid json file %s", path)
	}

	for _, entity := range allEntities {
		l.Set(entity)
	}

	return nil
}

func MakeLocker() Locker {
	ids := makeIDContainer()
	pos := makePosContainer()
	return Locker{
		byID:  &ids,
		byPos: &pos,
	}
}
