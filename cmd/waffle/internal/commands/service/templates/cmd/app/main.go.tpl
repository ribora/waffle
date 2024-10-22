package main

import (
	"{{ .Project }}/config"

	"gitlab.com/gorib/waffle/app"
)

func main() {
	config.Init()
	app.New().RunUntilStop()
}
