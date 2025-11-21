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
	goChan     chan string
	subscriber *nats.Subscription
}

type raftCommsStruct struct {
	natsCongig natsConfigType
	nc         *nats.Conn
	channels   map[string]channelType
}

var raftComms raftCommsStruct = raftCommsStruct{}

// ======================================================
// init initializes package-level variables
// ======================================================
func init() {
	raftComms.channels = make(map[string]channelType)

}

func GetInstance() *raftCommsStruct {
	return &raftComms
}

// ======================================================
// configStructure parses the JSON configuration string
// ======================================================
func configStructure(configJson string) error {
	var _err error = json.Unmarshal([]byte(configJson), &raftComms.natsCongig)
	if _err != nil {
		fmt.Printf("Error parsing config JSON: %v\n", _err)
	}
	return _err
}

// ======================================================
// config string is a JSON string that defines all the
// configuration parameters needed to initialize NATS
// ======================================================
func (raftCommsStruct) Init(config string) error {
	var _err error
	_ = configStructure(config)

	_err = configStructure(config)
	if _err == nil {
		// Connect to a server
		raftComms.nc, _err = nats.Connect(nats.DefaultURL)
	}
	if _err != nil {
		fmt.Printf("NATS connection error: %v\n", _err)
	}
	return _err
}

// ======================================================
// Close closes the NATS connection
// ======================================================
func (raftCommsStruct) Close() {
	var _err error
	for _, ch := range raftComms.channels {
		if ch.subscriber != nil {
			_err = ch.subscriber.Unsubscribe()
			if _err == nil {
				fmt.Printf("Unsubscribed from channel %s\n", ch.ID)
				_err = ch.subscriber.Drain()
				if _err == nil {
					fmt.Printf("Drained from channel %s\n", ch.ID)
					raftComms.nc.Close()
					fmt.Printf("nats connection closed %s\n", ch.ID)

				}
			}
		}
		if _err != nil {
			fmt.Printf("Error unsubscribing from channel %s: %v\n", ch.ID, _err)
		}
	}
	raftComms.nc.Close()
}

// ======================================================
// Send sends a message to a specified channel. This is
// different from publishing to a queue group and only.
// this message will go to all subscribers of the channel.
// ======================================================
func (raftCommsStruct) Send(message string, channel string) error {
	fmt.Println("Send message:", message)

	return raftComms.nc.Publish(channel, []byte(message))
}

// ======================================================
// Subscribe subscribes to a specified channel with a
// ======================================================
func (raftCommsStruct) Subscribe(channel string, goChan chan string) error {
	var _handler nats.MsgHandler = func(msg *nats.Msg) {
		_msg := string(msg.Data)
		goChan <- _msg
	}
	_subs, _err := raftComms.nc.Subscribe(channel, _handler)
	if _err == nil {
		raftComms.channels[channel] = channelType{
			ID:         channel,
			goChan:     goChan,
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
//
// ======================================================
func (raftCommsStruct) UnSubscribe(channel string) error {
	var _err error
	ch, exists := raftComms.channels[channel]
	if exists && ch.subscriber != nil {
		_err = ch.subscriber.Unsubscribe()
		if _err == nil {
			fmt.Printf("Unsubscribed from channel %s\n", channel)
			delete(raftComms.channels, channel)
		}
	} else {
		_err = fmt.Errorf("no subscription found for channel %s", channel)
	}
	if _err != nil {
		fmt.Printf("Error unsubscribing from channel %s: %v\n", channel, _err)
	}
	return _err
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
// handler for subscriber messages
// ======================================================
// func asyncMessageHandler(msg *nats.Msg) {
// 	fmt.Printf("Received a message: %s\n", string(msg.Data))
// }

// ======================================================
// listen subscribes to a specified channel with a
// ======================================================
// func listen(channel string, handler nats.MsgHandler) (*nats.Subscription, error) {
// 	return raftComms.nc.QueueSubscribe(channel, "queueGroup", handler)
// }
