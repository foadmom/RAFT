package comms

import (
	"encoding/json"
	"fmt"

	// "time"

	"github.com/nats-io/nats.go"
)

type natsConfigType struct {
	URL string `json:"url"`
}

var nc *nats.Conn
var natsConfig natsConfigType = natsConfigType{}

// ======================================================
//
// ======================================================
func init() {

}

// ======================================================
// configStructure parses the JSON configuration string
// ======================================================
func configStructure(configJson string) error {
	var _err error = json.Unmarshal([]byte(configJson), &natsConfig)
	if _err != nil {
		fmt.Printf("Error parsing config JSON: %v\n", _err)
	}
	return _err
}

// ======================================================
// config string is a JSON string that defines all the
// configuration parameters needed to initialize NATS
// ======================================================
func Init(config string) {
	var _err error
	_err = configStructure(config)
	if _err != nil {
		// Connect to a server
		nc, _err = nats.Connect(nats.DefaultURL)
	}
	if _err != nil {
		fmt.Printf("NATS connection error: %v\n", _err)
	}
}

func Close() {
	nc.Close()
}

func Send(message string, channel string) error {
	fmt.Println("Send message:", message)

	return nc.Publish("simplePublisher", []byte("Hello World"))
}
