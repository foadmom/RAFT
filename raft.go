package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"time"

	"github.com/foadmom/RAFT/comms"
	// "github.com/foadmom/RAFT/comms"
)

const (
	heartbeatChannel         = "ping"
	heartbeatResponseChannel = "pong"
	electionChannel          = "election"
)
const (
	heartbeatInterval        = 1000
	heartbeatTimeoutInterval = 3 * heartbeatInterval
	electionTimeoutInterval  = 10 * heartbeatInterval
)

const (
	notSet   = 0
	follower = 1
	leader   = 2
	election = 3
)

var nodeState int = notSet

// type pongMessage struct {
// 	followerID string `json:"followerID"` // this can be hostname/IP/or other identifier
// }

var statusMap map[string]string = make(map[string]string, 10)

// var heartbeatChan chan string = make(chan string, 10)

var processID string = strconv.FormatInt(int64(os.Getpid()), 16) // this should be unique per process in a real implementation

func main() {
	var _err error
	// Initialize NATS with the configuration

	_err = comms.Init(`{"url": "nats://localhost:4222"}`)
	if _err == nil {
		defer comms.Close()
		fmt.Println("NATS initialized successfully")
		// go leaderWorkflow()
		_err = followerWorkflow()
		if _err != nil {
			nodeState = electionWorkflow()
			if nodeState == leader {
				leaderWorkflow()
			} else if nodeState == follower {
				_ = followerWorkflow()
			}
		}
	}
	if _err != nil {
		fmt.Println("Error subscribing to heartbeatChannel channel:", _err)
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
func leaderWorkflow() {
	nodeState = leader
	var _err error
	var _message string
	var _responseChannel chan string = make(chan string, 10)
	var _HBTicker *time.Ticker = time.NewTicker(heartbeatInterval * time.Millisecond)
	var _TimeOutTicker *time.Ticker = time.NewTicker(heartbeatTimeoutInterval * time.Millisecond)

	_ = comms.UnSubscribe(heartbeatResponseChannel)                    // just in case it's already subscribed
	_err = comms.Subscribe(heartbeatResponseChannel, _responseChannel) // responses are ignored for now
	if _err == nil {
		for {
			// OuterLoop:
			// time.Sleep(heartbeatInterval)
			if _err == nil {
				fmt.Println("Sent:", _message)
				for {
					select {
					case resp := <-_responseChannel:
						fmt.Println("Received response:", resp)
						statusMap[resp] = "alive"
					case <-_HBTicker.C:
						fmt.Println("Heartbeat interval elapsed, reseting ticker.")
						_message = fmt.Sprintf("%v", time.Now().UTC())
						_ = comms.Send(_message, heartbeatChannel)
						_HBTicker.Reset(heartbeatInterval * time.Millisecond)
						// goto OuterLoop
					case <-_TimeOutTicker.C:
						fmt.Println("Hear_TimeOutTicker elapsed, reseting ticker.")
						// every once in a while all follower statuses are reset.
						// the followers that do not respond in the timeout interval will remain "unknown"
						resetFollowerStatus()
						_TimeOutTicker.Reset(heartbeatTimeoutInterval * time.Millisecond)
						// goto OuterLoop
					}
				}
			} else {
				break
			}
		}
	}
	if _err != nil {
		fmt.Println("Error sending heartbeat message:", _err)
	}
}

// ======================================================
// Reset all followers stati to unknown
// ======================================================
func resetFollowerStatus() {
	for k := range statusMap {
		statusMap[k] = "unknown"
	}
}

// ======================================================
// followerWorkflow monitors heartbeat messages and exits on timeout
// ======================================================
func followerWorkflow() error {
	var _err error
	_ = comms.UnSubscribe(electionChannel)
	_ = comms.UnSubscribe(heartbeatChannel)
	_ = comms.UnSubscribe(heartbeatResponseChannel)

	nodeState = follower
	var heartbeatGoChan chan string = make(chan string, 10)
	_err = comms.Subscribe(heartbeatChannel, heartbeatGoChan)

	if _err == nil {
		// var _err error
		var _ticker *time.Ticker = time.NewTicker(heartbeatTimeoutInterval)
		for {
			select {
			case msg := <-heartbeatGoChan:
				fmt.Println("Received heartbeat message:", msg)
				_ticker.Stop()
				_ = comms.Send(processID, heartbeatResponseChannel)
				_ticker = time.NewTicker(heartbeatTimeoutInterval)
			case <-_ticker.C:
				fmt.Printf("%v: No heartbeat message received in the last %v, exiting monitor.\n", time.Now(), heartbeatTimeoutInterval)
				nodeState = electionWorkflow()
				if nodeState == leader {
					leaderWorkflow()
				} else if nodeState == follower {
					_ = followerWorkflow()
				}
			}
		}
	}
	return _err
}

// ======================================================
// Election workflow. This occures when leader is suspected to be down
// and a new leader must be elected
// ======================================================
func electionWorkflow() int {
	nodeState = election
	fmt.Println("Starting election workflow")
	_ = comms.UnSubscribe(heartbeatChannel)
	_ = comms.UnSubscribe(heartbeatResponseChannel)
	_ = comms.UnSubscribe(electionChannel)

	var _electionGoChannel chan string = make(chan string, 10)
	_ = comms.Subscribe(electionChannel, _electionGoChannel)

	var _randonmTimer int = GenerateRandomInt(0, int(electionTimeoutInterval))
	var _ticker *time.Ticker = time.NewTicker((time.Duration(_randonmTimer) * time.Millisecond))

	for {
		select {
		case msg := <-_electionGoChannel:
			fmt.Println("Received election message:", msg)
			// another node has started an election, back to follower
			if msg == processID {
				fmt.Println(" WOWWWW   I am the leader")
				nodeState = leader
				return leader
			} else {
				fmt.Println(" Another node is leader, back to follower")
				nodeState = follower
				return follower
			}
		case <-_ticker.C:
			var _randonmTimer time.Duration = time.Duration(GenerateRandomInt(10, int(heartbeatTimeoutInterval))) * time.Millisecond
			_ticker.Reset(_randonmTimer)
			_ = comms.Send(processID, electionChannel)
			// timeout elapsed, become leader
		}
	}
	return notSet
}

// ========================================================
// the lower limit is inclusive and upper limit is excluded.
// so the random int is anything from lower to upper-1
// ========================================================
func GenerateRandomInt(lower int, upper int) int {
	var _rand int = rand.Intn(upper-lower) + lower
	return _rand
}
