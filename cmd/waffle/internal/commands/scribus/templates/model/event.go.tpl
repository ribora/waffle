//go:generate go tool whisk -validate.storage=pkg/validator $GOFILE

package model

type {{ title .Event }} struct {
	// Define payload schema here. Add `validate:"..."` tags as needed.
}
