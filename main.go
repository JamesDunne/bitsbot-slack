package main

import (
	"fmt"
	"log"
	"os"
)

var env map[string]string

func parseEnv(requiredEnv []string) error {
	// Get required environment variables:
	missing := make([]string, 0, len(requiredEnv))
	env = make(map[string]string)
	for _, name := range requiredEnv {
		value := os.Getenv(name)
		if value == "" {
			missing = append(missing, name)
		}
		env[name] = value
	}
	if len(missing) > 0 {
		return fmt.Errorf("Missing required environment variables: %v\n", missing)
	}
	return nil
}

func mainWebsocketClient() error {
	return nil
}

func main() {
	err := mainWebsocketClient()
	if err != nil {
		log.Println(err)
	}
}
