package state

import (
	"encoding/json"
	"sync"

	"github.com/clagraff/devoid/components"
	"github.com/clagraff/devoid/entities"

	errs "github.com/go-errors/errors"
	uuid "github.com/satori/go.uuid"
)

type State struct {
	entities  map[uuid.UUID]*entities.Entity
	positions map[components.Position]map[uuid.UUID]struct{}

	idLocks  map[uuid.UUID]*sync.Mutex
	posLocks map[components.Position]*sync.Mutex
}

func NewState() *State {
	return &State{
		entities:  make(map[uuid.UUID]*entities.Entity),
		positions: make(map[components.Position]map[uuid.UUID]struct{}),

		idLocks:  make(map[uuid.UUID]*sync.Mutex),
		posLocks: make(map[components.Position]*sync.Mutex),
	}
}

func (locker *State) ListIDs() []uuid.UUID {
	allIDs := make([]uuid.UUID, 0)
	for id := range locker.entities {
		allIDs = append(allIDs, id)
	}

	return allIDs
}

func (locker *State) Upsert(entity *entities.Entity) {
	lock, ok := locker.idLocks[entity.ID]
	if !ok {
		lock = new(sync.Mutex)
		locker.idLocks[entity.ID] = lock
	}
	lock.Lock()
	defer lock.Unlock()

	locker.entities[entity.ID] = entity

	posLock, ok := locker.posLocks[entity.Position]
	if !ok {
		posLock = new(sync.Mutex)
		locker.posLocks[entity.Position] = posLock
	}
	posLock.Lock()
	defer posLock.Unlock()

	if len(locker.positions[entity.Position]) == 0 {
		locker.positions[entity.Position] = make(map[uuid.UUID]struct{})
	}
	locker.positions[entity.Position][entity.ID] = struct{}{}
}

func (locker *State) UpsertPosition(entity *entities.Entity) {
	posLock, ok := locker.posLocks[entity.Position]
	if !ok {
		posLock = new(sync.Mutex)
		locker.posLocks[entity.Position] = posLock
	}
	posLock.Lock()
	defer posLock.Unlock()

	if len(locker.positions[entity.Position]) == 0 {
		locker.positions[entity.Position] = make(map[uuid.UUID]struct{})
	}
	locker.positions[entity.Position][entity.ID] = struct{}{}
}

func (locker *State) DeleteFromPosition(entity *entities.Entity) {
	posLock, ok := locker.posLocks[entity.Position]
	if !ok {
		posLock = new(sync.Mutex)
		locker.posLocks[entity.Position] = posLock
	}
	posLock.Lock()
	defer posLock.Unlock()

	if len(locker.positions[entity.Position]) == 0 {
		locker.positions[entity.Position] = make(map[uuid.UUID]struct{})
	}
	delete(locker.positions[entity.Position], entity.ID)
}

func (locker *State) DeleteIDFromPosition(id uuid.UUID, position components.Position) {
	posLock, ok := locker.posLocks[position]
	if !ok {
		posLock = new(sync.Mutex)
		locker.posLocks[position] = posLock
	}
	posLock.Lock()
	defer posLock.Unlock()

	if len(locker.positions[position]) == 0 {
		locker.positions[position] = make(map[uuid.UUID]struct{})
	}
	delete(locker.positions[position], id)
}

func (locker *State) ByID(id uuid.UUID) (*entities.Entity, func(), bool) {
	lock, ok := locker.idLocks[id]
	if !ok {
		return nil, nil, false
	}
	lock.Lock()

	entity, ok := locker.entities[id]
	if !ok {
		lock.Unlock()
		return nil, nil, false
	}

	return entity, lock.Unlock, true
}

func (locker *State) ByPosition(pos components.Position) ([]*entities.Entity, func(), bool) {
	emptyFn := func() {}
	var allUnlocks []func()

	lock, ok := locker.posLocks[pos]
	if !ok {
		return []*entities.Entity{}, emptyFn, true
	}
	lock.Lock()

	idMap, ok := locker.positions[pos]
	if !ok {
		lock.Unlock()
		return []*entities.Entity{}, emptyFn, true
	}

	allUnlocks = append(allUnlocks, lock.Unlock)
	entities := make([]*entities.Entity, len(idMap))
	i := 0

	for id, _ := range idMap {
		entityLock, ok := locker.idLocks[id]
		if !ok {
			for _, unlocker := range allUnlocks {
				unlocker()
			}
			return nil, nil, false
		}

		entityLock.Lock()
		allUnlocks = append(allUnlocks, entityLock.Unlock)

		entity, ok := locker.entities[id]
		if !ok {
			for _, unlocker := range allUnlocks {
				unlocker()
			}
			return nil, nil, false
		}

		entities[i] = entity
		i++
	}

	unlockAll := func() {
		for _, unlocker := range allUnlocks {
			unlocker()
		}
	}

	return entities, unlockAll, true
}

func (locker *State) FromBytes(input []byte) error {
	locker.entities = make(map[uuid.UUID]*entities.Entity)
	locker.positions = make(map[components.Position]map[uuid.UUID]struct{})
	locker.idLocks = make(map[uuid.UUID]*sync.Mutex)
	locker.posLocks = make(map[components.Position]*sync.Mutex)

	var desiredEntities []entities.Entity
	err := json.Unmarshal(input, &desiredEntities)
	if err != nil {
		return errs.New(err)
	}

	for _, entity := range desiredEntities {
		id := entity.ID
		pos := entity.Position

		e := new(entities.Entity)
		(*e) = entity
		locker.entities[id] = e

		if _, ok := locker.positions[pos]; !ok {
			locker.positions[pos] = make(map[uuid.UUID]struct{})
		}

		locker.positions[pos][id] = struct{}{}

		locker.idLocks[id] = new(sync.Mutex)
		locker.posLocks[pos] = new(sync.Mutex)
	}

	return nil
}

func (locker *State) ToBytes() ([]byte, error) {
	entities := make([]entities.Entity, len(locker.entities))
	i := 0

	for _, entity := range locker.entities {
		entities[i] = *entity
		i++
	}

	return json.Marshal(entities)
}
