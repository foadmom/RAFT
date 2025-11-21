package main

import (
	"fmt"
	"time"

	"github.com/foadmom/RAFT/comms"
	// "github.com/foadmom/RAFT/comms"
)

const (
	heartbeatChannel         = "ping"
	heartbeatResponseChannel = "pong"
	heartbeatInterval        = 100 * time.Millisecond
	heartbeatTimeoutFactor   = 3 * heartbeatInterval
)

var heartbeatChan chan string = make(chan string, 10)

func main() {
	var _err error
	// Initialize NATS with the configuration

	_err = comms.Init(`{"url": "nats://localhost:4222"}`)
	if _err == nil {
		defer comms.Close()
		fmt.Println("NATS initialized successfully")
		_err = comms.Subscribe(heartbeatChannel, heartbeatChan)
		sendHeartbeatMessages()
		monitorChannel(heartbeatChan)
	}
	if _err != nil {
		fmt.Println("Error subscribing to heartbeatChannel channel:", _err)
	}
	// Now you can use the comms package to interact with NATS
	fmt.Println("natsTest completed")
}

func sendHeartbeatMessages() {
	var _err error
	var _message string

	for i := 0; i < 3; i++ {
		_message = fmt.Sprintf("Heartbeat %d", i+1)
		_err = comms.Send(_message, heartbeatChannel)
		if _err == nil {
			fmt.Println("Sent:", _message)
			time.Sleep(1 * time.Second)
		}
	}
	if _err != nil {
		fmt.Println("Error sending heartbeat message:", _err)
	}
}

func monitorChannel(heartbeatChan <-chan string) {
	var _ticker *time.Ticker = time.NewTicker(3 * time.Second)
	for {
		select {
		case msg := <-heartbeatChan:
			fmt.Println("Received heartbeat message:", msg)
			_ticker.Stop()
			_ticker = time.NewTicker(3 * time.Second)
		case <-_ticker.C:
			fmt.Println("No heartbeat message received in the last 3 seconds, exiting monitor.")
			return
		}
	}
}
