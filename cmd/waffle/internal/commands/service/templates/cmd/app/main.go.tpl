package main

import (
	"fmt"
	"os"

	"{{ .Project }}/config"
)

func main() {
	application, err := config.Init()
	if err != nil {
		fmt.Printf("Failed to create an application: %v\n", err)
		os.Exit(1)
	}
	application.RunUntilStop()
}
