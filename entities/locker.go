package entities

import (
	"encoding/json"
	"io/ioutil"
	"sync"

	"github.com/clagraff/devoid/components"
	"github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

type Container interface {
	Get() Entity
	Set(Entity)
}

type container struct {
	mux    *sync.RWMutex
	entity *Entity
}

func (c container) Get() Entity {
	c.mux.RLock()
	defer c.mux.RUnlock()

	return *c.entity
}

func (c *container) Set(entity Entity) {
	c.mux.Lock()
	defer c.mux.Unlock()

	*c.entity = entity
}

func newContainer() Container {
	return &container{
		mux:    new(sync.RWMutex),
		entity: new(Entity),
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
func (c idContainer) All() []Entity {
	allContainers := make([]Entity, 0)

	ranger := func(key, value interface{}) bool {
		if ec, ok := value.(Container); ok {
			allContainers = append(allContainers, ec.Get())
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

func (l Locker) All() []Entity {
	return l.byID.All()
}

func (l Locker) GetByID(id uuid.UUID) (Entity, error) {
	container, ok := l.byID.Load(id)
	if !ok {
		return Entity{}, errors.Errorf("no entity with id %s", id)
	}

	return container.Get(), nil
}

func (l Locker) GetByPosition(pos components.Position) ([]Entity, error) {
	entitiesAtPosition, ok := l.byPos.Load(pos)
	if !ok {
		return nil, errors.Errorf("no position for %s", pos)
	}

	return entitiesAtPosition.All(), nil
}

func (l *Locker) Set(entity Entity) error {
	// Grab the entity and lock it.
	id := entity.ID

	container, ok := l.byID.Load(id)
	if !ok {
		container = newContainer()
		container.Set(entity)
	}

	// Grab old version. Check if position differs.

	oldEntity := container.Get()
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
	container.Set(entity)
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

	position := container.Get().Position

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
	currentEntities := l.byID.All()
	for _, e := range currentEntities {
		l.Delete(e.ID)
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
