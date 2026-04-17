// for alpine:
// # CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o ./bin/ ./...
// # sudo docker build -t raft --debug -f alpine_dockerfile .
// #
// FROM alpine:latest
// WORKDIR /app
// COPY ./bin/RAFT .
// COPY ./config.json .
// ENTRYPOINT ["/app/RAFT"]

// ==================================================================
// The basic flow is as a follower. When a follower suspects the leader is down,
// it goes into election mode. In election mode, it waits for
// a random amount of time and if it has been no other nomination,
// then puts itself forward as a candidate.
// If by the time it puts itself forward, no other node has put itself forward,
// then it becomes leader. If another node has put itself forward,
// then it votes for that node.
// If a node receives more than 50% of the votes, then it becomes leader
// and sends an inauguration message to all nodes.
// When a node receives an inauguration message, it becomes follower.
//
//	     ┌────────────┐
//	     │   Follower ◄─────┐
//	     └──────┬─────┘     │
//	            │           │
//	     ┌──────▼─────┐     │
//	     │  Election  │     │
//	     └──┬──────┬──┘     │
//	        │      │        │
//	┌───────▼─┐    └────────┤
//	│ Leader  │             │
//	└───────┬─┘             │
//	        │               │
//	        └───────────────┘
//
// ==================================================================
package main

import (
	"encoding/json"
	"fmt"
	"log"

	// "math/rand"
	"os"
	"strconv"
	"time"

	"github.com/foadmom/common/comms"
	c "github.com/foadmom/common/config"
	"github.com/foadmom/common/logger"
	u "github.com/foadmom/common/utils"
)

const TIME_FORMAT = "2006-01-02T15:04:05,000"

var maxNodeCount int = 5
var myNode nodeHost
var myNodeMode string

// for leader to keep track of nodes stati. the key is the node UniqueId
var statusMap map[string]genericMessage = make(map[string]genericMessage, 10)
var messageTemplate genericMessage // used to send heartbeat messages
var natsConfig comms.NatsConfig = comms.NatsConfig{}

// ==================================================================
// NodeHost represents a node in the cluster. It has a hostname, process ID and a unique ID.
// The unique ID is a combination of the hostname and process ID, which should be unique across the cluster.
// In a real implementation, you might want to use a more robust method of generating unique IDs,
// such as using a UUID library or a distributed ID generator like Snowflake.
// ==================================================================
func init() {
	initLogger()
	var _processID string = strconv.FormatInt(int64(os.Getpid()), 16) // this should be unique per process in a real implementation
	_hostName, _ := os.Hostname()
	fmt.Printf("RAFT's container hostname is %s\n", _hostName)

	_json, _err := c.FindKeyedSubJson("config.json", "Environment.Dev.Nats")
	if _err == nil {
		json.Unmarshal([]byte(_json), &natsConfig)
	} else {
		_err = nil
		log.Printf("Error reading NATS config: %v\n", _err)
		natsConfig.URL = "nats://localhost:4222" // default value
	}

	_uniqueId := fmt.Sprintf("%s-%s", _hostName, _processID)
	myNode = nodeHost{_hostName, _processID, _uniqueId}
	myNodeMode = UNKNOWN

	messageTemplate = genericMessage{heartbeatRequestId, "", myNode, ""}
}

// ======================================================
//
// ======================================================
func initLogger() {
	// initialize logger
	var _loggerConfig logger.Config = logger.Config{
		ConsoleLoggingEnabled: false,
		EncodeLogsAsJson:      false,
		FileLoggingEnabled:    true,
		Directory:             "/logs",
		// Directory:  "",
		Filename:   "raft.log",
		MaxSize:    10, // megabytes
		MaxBackups: 5,
		MaxAge:     30, // days
	}
	loggerInstance := logger.Instance()
	loggerInstance.Configure(_loggerConfig)
	logger.Instance().Print(logger.Trace, "application starting")
}

// ======================================================
//
// ======================================================
func main() {
	var _err error

	fmt.Println("Application staring")

	args := os.Args
	if len(args) > 1 {
		maxNodeCount, _ = strconv.Atoi(args[1])
	}
	fmt.Printf("Max node count set to %v\n", maxNodeCount)
	myNodeMode = FOLLOWER

	var connectionString string = fmt.Sprintf(`{"url": "%s"}`, natsConfig.URL)
	fmt.Printf("connecting to %s\n", connectionString)
	_err = comms.Init(connectionString)
	if _err == nil {
		// the main loop
		for {
			switch myNodeMode {
			case FOLLOWER:
				_err = followerWorkflow()
				if _err != nil {
					myNodeMode = ELECTION
				}
			case LEADER:
				_err = leaderWorkflow()
				if _err != nil {
					myNodeMode = FOLLOWER
				}
			case ELECTION:
				_err = electionWorkflow()
				if _err != nil {
					// if any error redo the election via going through FOLLOWER
					myNodeMode = FOLLOWER
				}
			default:
				myNodeMode = FOLLOWER
			}
		}
	}
	// Now you can use the comms package to interact with NATS
	fmt.Println("exiting with error: ", _err)
}

// ======================================================
// leaderWorkflow sends heartbeat messages periodically
// and awaits responses. Every timeout interval, it resets
// all follower statuses to "unknown"
// It runs indefinitely.
// ======================================================
func leaderWorkflow() error {
	myNodeMode = LEADER
	var _err error
	var _responseChannel chan []byte = make(chan []byte, 10)
	var _HBTicker *time.Ticker = time.NewTicker(heartbeatInterval * time.Millisecond)
	defer _HBTicker.Stop()
	var _TimeOutTicker *time.Ticker = time.NewTicker(heartbeatTimeoutInterval * time.Millisecond)
	defer _TimeOutTicker.Stop()

	unsubscribeAll()
	_err = comms.Subscribe(heartbeatResponseChannel, _responseChannel) // responses are ignored for now
	if _err == nil {
		_err = sendHeartbeatRequest()
	}
	if _err == nil {
		var _errCount int = 0
		for {
			select {
			case resp := <-_responseChannel:
				_TimeOutTicker.Reset(heartbeatTimeoutInterval * time.Millisecond)
				fmt.Println("Received response:", string(resp))
				var _msg genericMessage
				// convert response from json and update status map
				_msg, _err = jsonToMessage(resp)
				if _err == nil {
					setNodeStatus(_msg)
					_errCount = 0
				} else {
					_errCount++
					if _errCount > maxErrorCount {
						return _err
					}
				}
			case <-_HBTicker.C:
				fmt.Println("Heartbeat interval sending ping.")
				printStatusReport()

				// send heartbeat and wait for resp
				_err = sendRequest(heartbeatRequestId, "", heartbeatChannel)
				if _err == nil {
					_errCount = 0
				} else {
					_errCount++
					if _errCount > maxErrorCount {
						return _err
					}
				}
			case <-_TimeOutTicker.C:
				fmt.Println("We should not get here. Something has gone wrong. This means we have not received any responses for a while")
				printStatusReport()
				return _err
			}
		}
	}
	return _err
}

// ======================================================
// followerWorkflow waits for heartbeat messages from the leader and responds to them.
// If it does not receive a heartbeat message within the timeout interval, it assumes the leader is down
// and exits, which will trigger the election workflow in the main loop.
// Note: the follower would likely have more responsibilities such as responding to client requests,
// replicating logs, etc. This is a simplified version for demonstration purposes.
// ======================================================
func followerWorkflow() error {
	var _err error

	myNodeMode = FOLLOWER
	var heartbeatGoChan chan []byte = make(chan []byte, 10)
	_err = comms.Subscribe(heartbeatChannel, heartbeatGoChan)
	defer comms.UnSubscribe(heartbeatChannel)

	if _err == nil {
		// var _err error
		var _ticker *time.Ticker = time.NewTicker(heartbeatTimeoutInterval * time.Millisecond)
		defer _ticker.Stop()
		for {
			select {
			case _ = <-heartbeatGoChan:
				// fmt.Println("Received heartbeat message:", string(msg))
				_ticker.Reset(heartbeatTimeoutInterval * time.Millisecond)
				_err = sendRequest(heartbeatResponseId, HEALTHY, heartbeatResponseChannel)
				if _err != nil {
					// this should never happen as send can only fail if messaging is down
					return _err
				}
			case <-_ticker.C:
				fmt.Printf("%v: No heartbeat message received in the last %v, exiting monitor.\n", time.Now(), heartbeatTimeoutInterval)
				_err = fmt.Errorf("no resolution of the election within the specified period")
				return _err
			}
		}
	}
	return _err
}

// ======================================================
// electionWorkflow handles the election process when a follower suspects
// the leader is down. It waits for a random amount of time and if
// no other node has put itself forward, it puts itself forward
// as a candidate. If another node has put itself forward,
// it votes for that node. If a node receives more than 50% of the votes,
// it becomes leader and sends an inauguration message to all nodes.
// When a node receives an inauguration message, it becomes follower.
// Note: In a real implementation, the election process would
// likely be more sophisticated and would handle edge cases
// such as split votes, network partitions, etc.
// This is a simplified version for demonstration purposes.
// ======================================================
func electionWorkflow() error {
	var _err error
	myNodeMode = ELECTION
	fmt.Println("Starting election workflow")

	var _electionGoChannel chan []byte = make(chan []byte, 10)
	_err = comms.Subscribe(electionChannel, _electionGoChannel)
	if _err == nil {
		defer comms.UnSubscribe(electionChannel)

		// every node will backoff for a random amount of time.
		// If by then no node has put itself forward for leader, then I will put myself forward
		var _randonmTimer int = u.GenerateRandomInt(0, int(electionTimeoutInterval))
		var _randomTimeout *time.Timer = time.NewTimer((time.Duration(_randonmTimer) * time.Millisecond))
		defer _randomTimeout.Stop()
		// we also need a timeout where we call it quits because election has not produced a leader
		var _timeout *time.Timer = time.NewTimer((electionTimeoutInterval * time.Millisecond))
		defer _timeout.Stop()
		var _noOfVotes int = 0
	LOOP:
		for {
			select {
			case _msg := <-_electionGoChannel:
				_randomTimeout.Stop() // now we wait for voting to finish
				var _electionMessage genericMessage
				_err = json.Unmarshal(_msg, &_electionMessage)
				// fmt.Printf("*** Election message received %v\n", _electionMessage)
				if _err == nil {
					switch _electionMessage.RequestId {
					case nominationtRequestId:
						if _electionMessage.Sender.UniqueId == myNode.UniqueId { // this is my nomination message, I can ignore it
							continue
						} else {
							_randomTimeout.Stop() // now we wait for voting to finish
							_err = sendRequest(electionVoteId, _electionMessage.Sender.UniqueId, electionChannel)
						}
					case electionVoteId:
						_candidateId := _electionMessage.Data
						if _candidateId == myNode.UniqueId {
							_noOfVotes++

							if _noOfVotes > (maxNodeCount / 2) {
								_err = sendRequest(inaugurationRequestId, myNode.UniqueId, electionChannel)
								myNodeMode = LEADER
								break LOOP
							}
						}
					case inaugurationRequestId:
						fmt.Println(" New leader has been inaugurated, I become follower")
						myNodeMode = FOLLOWER
						break LOOP
					}
				} else {
					fmt.Println("Error converting election message from json: ", _err)
				}
			case <-_randomTimeout.C:
				_err = sendRequest(nominationtRequestId, myNode.UniqueId, electionChannel)
				if _err != nil {
					break LOOP
				}
				_noOfVotes++ // I vote for myself
			case <-_timeout.C:
				// This should not happen
				_err = fmt.Errorf("no resolution of the election within the specified period\n")
				break LOOP
			}

		}
	}
	if _err != nil {
		fmt.Println("electionWorkflow: Error: ", _err)
	}
	return _err
}

// ======================================================
// generateMessage creates a genericMessage struct and converts it to json byte array.
// ======================================================
func generateMessage(requestID string, data string) ([]byte, error) {
	var _time string = time.Now().Format(TIME_FORMAT)
	messageTemplate.Timestamp = _time
	messageTemplate.RequestId = requestID
	messageTemplate.Data = data
	_jrequest, _err := json.Marshal(messageTemplate)
	return _jrequest, _err
}

// ======================================================
// sendRequest is a helper function to send a message to a channel.
// It generates the message based on the requestId and data
// provided and then sends it to the specified channel.
// It returns an error if there is an issue with generating the message or sending it.
// This function is used by both the leader and follower workflows
// to send messages to each other. // It abstracts away the details of
// message generation and sending, making the code cleaner and more maintainable.
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
// This is a heartbeat send to all followers.
// It is used by the leader to check if followers are alive
// ======================================================
func sendHeartbeatRequest() error {
	return sendRequest(heartbeatRequestId, "", heartbeatChannel)
}

// ======================================================
// unsubscribeAll unsubscribes from all channels.
// This is used when changing modes to avoid receiving
// messages that are not relevant to the current mode
// ======================================================
func unsubscribeAll() {

	_ = comms.UnSubscribe(electionChannel)
	_ = comms.UnSubscribe(heartbeatChannel)
	_ = comms.UnSubscribe(heartbeatResponseChannel)
}

// ======================================================
// setNodeStatus updates the status of a node in the status map.
// The status is based on the data field of the message, which can be "healthy" or "dead".
// storing the last Message because it contains the timestamp
// which is used to determine if the node is dead or not.
// ======================================================
func setNodeStatus(message genericMessage) {
	statusMap[message.Sender.UniqueId] = message
}

// ======================================================
// jsonToMessage converts a json byte array to a genericMessage struct.
// ======================================================
func jsonToMessage(jMsg []byte) (genericMessage, error) {
	var _genericMessage genericMessage
	_err := json.Unmarshal(jMsg, &_genericMessage)
	return _genericMessage, _err
}

// ======================================================
// printStatusReport prints the status of all nodes in the status map.
// It also checks if any node has not sent a heartbeat response
// within the timeout interval and marks it as "dead".
// This is used by the leader to monitor the health of the followers.
// Note: In a real implementation, this would likely be more
// sophisticated and would not print to the console.
// It is used here for demonstration purposes.
// The status report includes the node unique ID, its status (healthy or dead),
// and the timestamp of the last heartbeat response received.
// The timestamp is used to determine if the node is dead or
// not based on the heartbeat timeout interval.
// ======================================================
func printStatusReport() {
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
