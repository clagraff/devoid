package entities

import (
	"bitbucket.org/clagraff/yawning/components"

	uuid "github.com/satori/go.uuid"
)

type Entity struct {
	ID uuid.UUID

	Position components.Position
	Spatial  components.Spatial
}
