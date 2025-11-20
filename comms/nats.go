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

type channelType struct {
	ID         string
	channel    chan string
	subscriber *nats.Subscription
}

var channels map[string]channelType = make(map[string]channelType)

var nc *nats.Conn
var natsConfig natsConfigType = natsConfigType{}

// ======================================================
// init initializes package-level variables
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

// ======================================================
// Close closes the NATS connection
// ======================================================
func Close() {
	nc.Close()
}

// ======================================================
// Send sends a message to a specified channel. This is
// different from publishing to a queue group and only.
// this message will go to all subscribers of the channel.
// ======================================================
func Send(message string, channel string) error {
	fmt.Println("Send message:", message)

	return nc.Publish("simplePublisher", []byte(message))
}

// ======================================================
// ======================================================
// ======================================================
// ======================================================
// ======================================================
// ======================================================
// ======================================================
// ======================================================
// ======================================================
// ======================================================
// ======================================================
// ======================================================
// ======================================================
// ======================================================
// ======================================================
// Subscribe subscribes to a specified channel with a
// ======================================================
func Subscribe(channel string, subChannel chan string) error {
	var _handler nats.MsgHandler = func(msg *nats.Msg) {
		subChannel <- string(msg.Data)
	}
	_subs, _err := nc.Subscribe(channel, _handler)
	if _err == nil {
		channels[channel] = channelType{
			ID:         channel,
			channel:    subChannel,
			subscriber: _subs,
		}
	}
	if _err != nil {
		fmt.Printf("Subscribe error: %v\n", _err)
		return _err
	}

	return _err
}

// ======================================================
// handler for subscriber messages
// ======================================================
func asyncMessageHandler(msg *nats.Msg) {
	fmt.Printf("Received a message: %s\n", string(msg.Data))
}

// ======================================================
// listen subscribes to a specified channel with a
// ======================================================
func listen(channel string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return nc.QueueSubscribe(channel, "queueGroup", handler)
}
