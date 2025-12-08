package comms

type commsInterface interface {
	Init(config string) error
	Close()
	Send(message []byte, channel string) error
	Subscribe(channel string, goChannel chan []byte) error
	UnSubscribe(channel string) error
	Cleanup() error
}

var Comms commsInterface
var gConfig string

func init() {
	Comms = GetInstance()
}

func Init(config string) error {
	gConfig = config
	return Comms.Init(gConfig)
}

func Close() {
	Comms.Close()
}

func Send(message []byte, channel string) error {
	var _err error = Comms.Send(message, channel)
	if _err != nil {
		_err = Init(gConfig)
		if _err == nil {
			_err = Comms.Send(message, channel)
		}
	}
	return _err
}

func Subscribe(channel string, goChan chan []byte) error {
	return Comms.Subscribe(channel, goChan)
}

func UnSubscribe(channel string) error {
	return Comms.UnSubscribe(channel)
}
