package intents

import (
	"encoding/json"
	"testing"

	"github.com/clagraff/devoid/components"
	uuid "github.com/satori/go.uuid"
)

func setupValidIntent(t *testing.T) (string, []byte) {
	move := Move{
		SourceID: uuid.Must(uuid.NewV4()),
		Position: components.Position{},
	}

	marshalled, err := json.Marshal(move)
	if err != nil {
		t.Fatal(err)
	}

	validKind := "intents.Move"

	return validKind, marshalled
}

func TestUnmarshal(t *testing.T) {
	t.Run("unmarshalling valid intent returns no error", func(t *testing.T) {
		validKind, validBytes := setupValidIntent(t)

		_, err := Unmarshal(validKind, validBytes)
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("unmarshalling unknown intent returns error", func(t *testing.T) {
		invalidKind := "unknownIntentType"
		_, validBytes := setupValidIntent(t)

		_, err := Unmarshal(invalidKind, validBytes)
		if err == nil {
			t.Error("expected an error, but received nil")
		}
	})

	t.Run("unmarshalling invalid bytes returns error", func(t *testing.T) {
		validKind, _ := setupValidIntent(t)
		invalidBytes := []byte("this is not valid json")

		_, err := Unmarshal(validKind, invalidBytes)
		if err == nil {
			t.Error("expected an error, but received nil")
		}
	})
}
