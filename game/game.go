package game

import (
	"sync"

	uuid "github.com/satori/go.uuid"
)

type Entity struct {
	ID uuid.UUID

	Position Position
	//	Spatial  Spatial
}

func (entity Entity) String() string {
	return "entity"
}

// type Spatial struct {
// 	OccupiesPosition bool
// 	Stackable        bool
// }

type Position struct {
	X int
	Y int
}

type EntityLocker struct {
	entity *Entity
	mutex  *sync.Mutex
}

func (locker *EntityLocker) Lock() *Entity {
	locker.mutex.Lock()
	return locker.entity
}

func (locker *EntityLocker) Unlock() {
	locker.mutex.Unlock()
}

func NewEntityLocker(entity *Entity) *EntityLocker {
	return &EntityLocker{
		entity: entity,
		mutex:  new(sync.Mutex),
	}
}

type EntityCache map[uuid.UUID]*EntityLocker

type PositionCache map[Position][]*EntityLocker
