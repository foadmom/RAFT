package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
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

// ==================================================================
//
// ==================================================================
func init() {
	var _processID string = strconv.FormatInt(int64(os.Getpid()), 16) // this should be unique per process in a real implementation
	_hostName, _ := os.Hostname()
	fmt.Printf("RAFT's container hostname is %s\n", _hostName)

	_uniqueId := fmt.Sprintf("%s-%s", _hostName, _processID)
	myNode = nodeHost{_hostName, _processID, _uniqueId}
	myNodeMode = UNKNOWN

	messageTemplate = genericMessage{heartbeatRequestId, "", myNode, ""}
}

// ==================================================================
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
func hostnames(hostname string) {
	var err error
	addrs, err := net.LookupIP(hostname) // returns a slice of the IP addresses of the host
	// lookupIP looks up host using the local resolver. It returns a slice of that host's IPv4 and IPv6 addresses.
	if err != nil {
		log.Println("Failed to detect machine host name. ", err.Error())
		return
	}
	log.Printf("hostname %s Addrs: %v", hostname, addrs)
}

// ======================================================
// code provided by: Rustavil Nurkaev
// This is just to test connectivity to the other nodes in the cluster.
// ======================================================
func RawConnect(host string, ports []string) {
	for _, port := range ports {
		timeout := time.Second
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), timeout)
		if err != nil {
			fmt.Println("Connecting error:", err)
		}
		if conn != nil {
			defer conn.Close()
			fmt.Println("Opened", net.JoinHostPort(host, port))
		} else {
			fmt.Println("Failed to connect to", net.JoinHostPort(host, port))
		}
	}
}

// ======================================================
//
// ======================================================
func main() {
	var _err error
	// Initialize NATS with the configuration
	myNodeMode = FOLLOWER
	// hostnames("nats")
	var hostname string = "nats-server"
	addrs, err := net.LookupIP(hostname)
	nats_address := fmt.Sprintf("%s", addrs[0])
	if err == nil {
		fmt.Printf("Host %s has the following addresses: %s\n", hostname, nats_address)
	} else {
		fmt.Printf("Failed to lookup IP addresses for host %s: %v\n", hostname, err)
	}
	RawConnect(hostname, []string{"4222"})
	RawConnect(nats_address, []string{"4222"})
	// _err = comms.Init(`{"url": "nats://localhost:4222"}`)
	var connectionString string = fmt.Sprintf(`{"url": "nats://%s:4222"}`, nats_address)
	fmt.Printf("the connection string is %s\n", connectionString)
	_err = comms.Init(connectionString)
	// _err = comms.Init(`{"url": "nats://nats-server:4222"}`)
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
				_msg, _err = convertMessageFromJson(resp)
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
				printStati()
				return _err
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
			case msg := <-heartbeatGoChan:
				fmt.Println("Received heartbeat message:", string(msg))
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
// Election workflow. This occures when leader is suspected to be down
// and a new leader must be elected
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
		var _randonmTimer int = GenerateRandomInt(0, int(electionTimeoutInterval))
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
				fmt.Println("Received election message:", string(_msg))
				_err = json.Unmarshal(_msg, &_electionMessage)
				if _err == nil {
					switch _electionMessage.RequestId {
					case nominationtRequestId:
						fmt.Println(" Another node has requested my vote, I vote for it and become follower")
						_err = sendRequest(electionVoteId, _electionMessage.Sender.UniqueId, electionChannel)
						if _err != nil {
							break LOOP
						}
					case electionVoteId:
						if _electionMessage.Data == myNode.UniqueId {
							fmt.Println(" I have received a vote for my leadership")
							_noOfVotes++
							if _noOfVotes > (len(statusMap) / 2) {
								fmt.Println(" I have received majority votes, I become leader")
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
				}
			case <-_randomTimeout.C:
				_err = sendRequest(nominationtRequestId, myNode.UniqueId, electionChannel)
				if _err != nil {
					break LOOP
				}
			case <-_timeout.C:
				// This should not happen
				_err = fmt.Errorf("no resolution of the election within the specified period")
				break LOOP
			}

		}
	}
	if _err != nil {
		fmt.Println("electionWorkflow: Error: ", _err)
	}
	return _err
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
