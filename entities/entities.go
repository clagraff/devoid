package entities

import (
	"github.com/clagraff/devoid/components"

	uuid "github.com/satori/go.uuid"
)

type Entity struct {
	ID uuid.UUID

	Position components.Position
	Spatial  components.Spatial
}
