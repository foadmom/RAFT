package comms

type commsInterface interface {
	Init(config string) error
	Close()
	Send(message string, channel string) error
	Subscribe(channel string, subChannel chan string) error
	UnSubscribe(channel string) error
}

var Comms commsInterface

func init() {
	Comms = GetInstance()
}

func Init(config string) error {
	return Comms.Init(config)
}

func Close() {
	Comms.Close()
}

func Send(message string, channel string) error {
	return Comms.Send(message, channel)
}

func Subscribe(channel string, goChan chan string) error {
	return Comms.Subscribe(channel, goChan)
}

func UnSubscribe(channel string) error {
	return Comms.UnSubscribe(channel)
}
