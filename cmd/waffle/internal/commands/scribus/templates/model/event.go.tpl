//go:generate go tool gorival -input $GOFILE -storage pkg/validator

package model

type {{ title .Event }} struct {
	// Define payload schema here. Add `validate:"..."` tags as needed.
}
