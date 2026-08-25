//go:generate go tool whisk $GOFILE

package model

type {{ title .Event }} struct {
	// Define payload schema here. Add `validate:"..."` tags as needed.
}
