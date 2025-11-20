package main

import (
	"fmt"

	"github.com/foadmom/RAFT/comms"
	// "github.com/foadmom/RAFT/comms"
)

func main() {
	// Example configuration JSON string
	configJson := `{"url": "nats://localhost:4222"}`

	// Initialize NATS with the configuration
	comms.Init(configJson)

	// Now you can use the comms package to interact with NATS
	fmt.Println("NATS initialized with config:", configJson)
}
