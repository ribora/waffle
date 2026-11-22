package config

import (
	"github.com/ribora/waffle/app"
	"{{ .Project }}/config/diogen"
)

func Init() (app.App, error) {
	return diogen.InitDi()
}
