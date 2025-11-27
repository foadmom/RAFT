package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/foadmom/RAFT/comms"
	// "github.com/foadmom/RAFT/comms"
)

const TIME_FORMAT = "2006-01-02T15:04:05,000"

var myNode nodeHost
var myNodeMode string

// for leader to keep track of nodes stati. the key is the node UniqueId
var statusMap map[string]genericMessage = make(map[string]genericMessage, 10)
var messageTemplate genericMessage // used to send heartbeat messages

func init() {
	var _processID string = strconv.FormatInt(int64(os.Getpid()), 16) // this should be unique per process in a real implementation
	_hostName, _ := os.Hostname()
	_uniqueId := fmt.Sprintf("%s-%s", _hostName, _processID)
	myNode = nodeHost{_hostName, _processID, _uniqueId}
	myNodeMode = UNKNOWN

	messageTemplate = genericMessage{heartbeatRequestId, "", myNode, ""}
}

func main() {
	var _err error
	// Initialize NATS with the configuration
	myNodeMode = FOLLOWER
	_err = comms.Init(`{"url": "nats://localhost:4222"}`)
	if _err == nil {
		// the main loop
		for {
			switch myNodeMode {
			case FOLLOWER:
				_ = followerWorkflow()
			case LEADER:
				_ = leaderWorkflow()
			case ELECTION:
				_ = electionWorkflow()
			default:
				myNodeMode = FOLLOWER
			}
		}
	}
	// Now you can use the comms package to interact with NATS
	fmt.Println("natsTest completed")
}

// ======================================================
// leaderWorkflow sends heartbeat messages periodically
// and awaits responses. Every timeout interval, it resets
// all follower statuses to "unknown"
// It runs indefinitely.
// Note: In a real implementation, there would be a way to
// stop this goroutine gracefully.
// ======================================================
func leaderWorkflow() error {
	myNodeMode = LEADER
	var _err error
	var _responseChannel chan []byte = make(chan []byte, 10)
	var _HBTicker *time.Ticker = time.NewTicker(heartbeatInterval * time.Millisecond)
	var _TimeOutTicker *time.Ticker = time.NewTicker(heartbeatTimeoutInterval * time.Millisecond)

	unsubscribeAll()
	_err = comms.Subscribe(heartbeatResponseChannel, _responseChannel) // responses are ignored for now
	if _err == nil {
		_err = sendHeartbeatRequest()
	}
	if _err == nil {
		// loop:
		for {
			select {
			case resp := <-_responseChannel:
				fmt.Println("Received response:", string(resp))
				var _msg genericMessage
				// convert response from json and update status map
				_msg, _err = convertMessageFromJson(resp)
				if _err == nil {
					setNodeStatus(_msg)
				}
			case <-_HBTicker.C:
				fmt.Println("Heartbeat interval elapsed, reseting ticker.")
				// send heartbeat and wait for resp
				_err = sendRequest(heartbeatRequestId, "", heartbeatChannel)
				if _err == nil {
					_HBTicker.Reset(heartbeatInterval * time.Millisecond)
					// _TimeOutTicker.Reset(heartbeatTimeoutInterval * time.Millisecond)
				}
				// goto OuterLoop
			case <-_TimeOutTicker.C:
				fmt.Println("Hear_TimeOutTicker elapsed.")
				printStati()
				_TimeOutTicker.Reset(heartbeatTimeoutInterval * time.Millisecond)
				// every once in a while all follower statuses are reset.
				// the followers that do not respond in the timeout interval will remain "unknown"
				// resetAllStati()
			}
		}
	}
	return _err
}

// ======================================================
// followerWorkflow monitors heartbeat messages and exits on timeout
// ======================================================
func followerWorkflow() error {
	var _err error
	unsubscribeAll()

	myNodeMode = FOLLOWER
	var heartbeatGoChan chan []byte = make(chan []byte, 10)
	_err = comms.Subscribe(heartbeatChannel, heartbeatGoChan)

	if _err == nil {
		// var _err error
		var _ticker *time.Ticker = time.NewTicker(heartbeatTimeoutInterval * time.Millisecond)
	loop:
		for {
			select {
			case msg := <-heartbeatGoChan:
				fmt.Println("Received heartbeat message:", string(msg))
				_ticker.Reset(heartbeatTimeoutInterval * time.Millisecond)
				_err = sendRequest(heartbeatResponseId, HEALTHY, heartbeatResponseChannel)
				if _err != nil {
					myNodeMode = UNKNOWN
					break loop
				}
			case <-_ticker.C:
				fmt.Printf("%v: No heartbeat message received in the last %v, exiting monitor.\n", time.Now(), heartbeatTimeoutInterval)
				myNodeMode = ELECTION
				break loop
			}
		}
	}
	if _err != nil {
		fmt.Println("followerWorkflow: ", _err)
	}
	return _err
}

// ======================================================
// Election workflow. This occures when leader is suspected to be down
// and a new leader must be elected
// ======================================================
func electionWorkflow() error {
	var _err error
	myNodeMode = ELECTION
	fmt.Println("Starting election workflow")
	unsubscribeAll()

	var _electionGoChannel chan []byte = make(chan []byte, 10)
	_err = comms.Subscribe(electionChannel, _electionGoChannel)
	if _err != nil {
		fmt.Println("Error subscribing to election channel:", _err)
		return _err
	}

	var _randonmTimer int = GenerateRandomInt(0, int(electionTimeoutInterval))
	var _ticker *time.Ticker = time.NewTicker((time.Duration(_randonmTimer) * time.Millisecond))
	var _noOfVotes int = 0
	for {
		select {
		case _msg := <-_electionGoChannel:
			var _electionMessage genericMessage
			fmt.Println("Received election message:", string(_msg))
			_err = json.Unmarshal(_msg, &_electionMessage)
			if _err != nil {
				fmt.Println("Error unmarshaling election message:", _err)
				continue
			} else {
				switch _electionMessage.RequestId {
				case nominationtRequestId:
					fmt.Println(" Another node has requested my vote, I vote for it and become follower")
					_ticker.Reset(electionTimeoutInterval) // in case I wait and get nothing
					sendRequest(electionVoteId, _electionMessage.Sender.UniqueId, electionChannel)
					continue // wait for votes
				case electionVoteId:
					// _ticker.Reset(electionTimeoutInterval) // in case I wait and get nothing
					if _electionMessage.Data == myNode.UniqueId {
						fmt.Println(" I have received a vote for my leadership")
						_noOfVotes++
						if _noOfVotes > (len(statusMap) / 2) {
							fmt.Println(" I have received majority votes, I become leader")
							sendRequest(inaugurationRequestId, myNode.UniqueId, electionChannel)
							myNodeMode = LEADER
							return nil
						} else {
							continue // wait for more votes
						}
					}
				case inaugurationRequestId:
					fmt.Println(" New leader has been inaugurated, I become follower")
					_ticker.Stop()
					myNodeMode = FOLLOWER
					return nil
				}
			}
		case <-_ticker.C:
			var _randonmTimer time.Duration = time.Duration(GenerateRandomInt(10, int(heartbeatTimeoutInterval))) * time.Millisecond
			_ticker.Reset(_randonmTimer)
			_ = sendRequest(nominationtRequestId, myNode.UniqueId, electionChannel)
			// timeout elapsed, become leader
		}

	}
}

// ========================================================
// the lower limit is inclusive and upper limit is excluded.
// so the random int is anything from lower to upper-1
// ========================================================
func GenerateRandomInt(lower int, upper int) int {
	var _rand int = rand.Intn(upper-lower) + lower
	return _rand
}

// ======================================================
//
// ======================================================
func generateMessage(requestID string, data string) ([]byte, error) {
	var _time string = time.Now().Format(TIME_FORMAT)
	// var _time string = string(time.Now().UnixMilli())
	messageTemplate.Timestamp = _time
	messageTemplate.RequestId = requestID
	messageTemplate.Data = data
	_jrequest, _err := json.Marshal(messageTemplate)
	return _jrequest, _err
}

// ======================================================
//
// ======================================================
func sendRequest(requestId string, data string, channel string) error {
	var _err error
	var _request []byte
	_request, _err = generateMessage(requestId, data)
	if _err == nil {
		_err = comms.Send(_request, channel)
	}
	return _err
}

// ======================================================
// sendRequest(requestId string, data string, channel string)
// ======================================================
func sendHeartbeatRequest() error {
	return sendRequest(heartbeatRequestId, "", heartbeatChannel)
}

// ======================================================
//
// ======================================================
func unsubscribeAll() {
	_ = comms.UnSubscribe(electionChannel)
	_ = comms.UnSubscribe(heartbeatChannel)
	_ = comms.UnSubscribe(heartbeatResponseChannel)
}

// ======================================================
//
// ======================================================
func setNodeStatus(message genericMessage) {
	statusMap[message.Sender.UniqueId] = message
}

// ======================================================
//
// ======================================================
func convertMessageFromJson(msg []byte) (genericMessage, error) {
	var _genericMessage genericMessage
	_err := json.Unmarshal(msg, &_genericMessage)
	return _genericMessage, _err
}

// ======================================================
//
// ======================================================
func printStati() {
	var _now int64 = time.Now().UnixMilli()

	for k, v := range statusMap {
		// var  _timestamp time.Time
		_timestamp, _ := time.Parse(TIME_FORMAT, v.Timestamp)
		var _diff int64 = _now - _timestamp.UnixMilli()
		if _diff > heartbeatTimeoutInterval {
			v.Data = DEAD
			statusMap[k] = v
		}
		fmt.Printf(" ============== %s:%s  %s\n", k, v.Data, v.Timestamp)
	}
}
