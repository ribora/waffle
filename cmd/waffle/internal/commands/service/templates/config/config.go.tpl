package config

import (
	"gitlab.com/gorib/waffle/app"
	"{{ .Project }}/config/diogen"
)

func Init() (app.App, error) {
	return diogen.InitDi()
}
