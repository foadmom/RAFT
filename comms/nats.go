package comms

// https://pkg.go.dev/github.com/nats-io/nats.go#section-readme
import (
	"encoding/json"
	"fmt"

	// "time"

	"github.com/nats-io/nats.go"
)

type NatsConfig struct {
	URL            string `json:"URL"`
	ClusterPort    string `json:"ClusterPort"`
	ClusterName    string `json:"ClusterName"`
	ManagementPort string `json:"ManagementPort"`
}

type channelType struct {
	ID         string
	GoChan     chan []byte
	Subscriber *nats.Subscription
}

type raftCommsStruct struct {
	NatsCongig NatsConfig
	Nc         *nats.Conn
	Channels   map[string]channelType
}

var raftComms raftCommsStruct = raftCommsStruct{}

// ======================================================
// init initializes package-level variables
// ======================================================
func init() {
	raftComms.Channels = make(map[string]channelType)

}

func GetInstance() *raftCommsStruct {
	return &raftComms
}

// ======================================================
// configStructure parses the JSON configuration string
// ======================================================
func configStructure(configJson string) error {
	var _err error = json.Unmarshal([]byte(configJson), &raftComms.NatsCongig)
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
	// _ = configStructure(config)

	_err = configStructure(config)
	if _err == nil {
		raftComms.Cleanup()
		// Connect to a server
		if raftComms.Nc != nil {
			raftComms.Close()
		}
		// raftComms.nc, _err = nats.Connect(nats.DefaultURL)
		raftComms.Nc, _err = nats.Connect(raftComms.NatsCongig.URL)
	}
	if _err != nil {
		fmt.Printf("NATS connection error to %s: %v\n", raftComms.NatsCongig.URL, _err)
	}
	return _err
}

// ======================================================
// Close closes the NATS connection
// ======================================================
func (raftCommsStruct) Close() {
	raftComms.Cleanup()
	if raftComms.Nc != nil {
		raftComms.Nc.Close()
		fmt.Printf("nats connection closed.\n")
	}
}

// ======================================================
// Send sends a message to a specified channel. This is
// different from publishing to a queue group and only.
// this message will go to all subscribers of the channel.
// ======================================================
func (raftCommsStruct) Send(message []byte, channel string) error {
	fmt.Println("Send message:", string(message))

	return raftComms.Nc.Publish(channel, message)
}

// ======================================================
// Subscribe subscribes to a specified channel with a
// ======================================================
func (raftCommsStruct) Subscribe(channel string, goChan chan []byte) error {
	var _handler nats.MsgHandler = func(msg *nats.Msg) {
		goChan <- msg.Data
	}
	_subs, _err := raftComms.Nc.Subscribe(channel, _handler)
	if _err == nil {
		raftComms.Channels[channel] = channelType{
			ID:         channel,
			GoChan:     goChan,
			Subscriber: _subs,
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
	ch, exists := raftComms.Channels[channel]
	if exists && ch.Subscriber != nil {
		_err = ch.Subscriber.Unsubscribe()
		if _err == nil {
			fmt.Printf("Unsubscribed from channel %s\n", channel)
			delete(raftComms.Channels, channel)
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
//
// ======================================================
func (raftCommsStruct) Cleanup() error {
	var _err error
	if raftComms.Nc != nil {
		for _key, _channel := range raftComms.Channels {
			_ = _channel.Subscriber.Unsubscribe()
			_channel.Subscriber.Drain()
			fmt.Printf("Unsubscribed from channel %s\n", _key)
		}
	}
	// clear the established channel infos
	raftComms.Channels = make(map[string]channelType)
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
