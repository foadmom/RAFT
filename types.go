package main

// heartbeat message (heartbeatRequestId)           standard message
// heartbeatResponse message (heartbeatResponseId)  standard message with status in Data
// nomination message (nominationtId)               standard message with host id in Data
// vote message (electionVoteId)                    standard message with candidate ID in Data
// inaugration message (inaugurationId)             standard message with new leader ID in Data

// ======================================================
// Constants for requestIds in the genericMessage field of
// type genericMessage
// ======================================================
const (
	heartbeatRequestId    = "ping"
	heartbeatResponseId   = "pong"
	nominationtRequestId  = "nomination"
	electionVoteId        = "myVote"
	inaugurationRequestId = "inauguration"
)

// ======================================================
// Communication channels
// ======================================================
const (
	heartbeatChannel         = heartbeatRequestId
	heartbeatResponseChannel = heartbeatResponseId
	electionChannel          = "election"
)
const (
	heartbeatInterval        = 1000
	heartbeatTimeoutInterval = 3 * heartbeatInterval
	electionTimeoutInterval  = 10 * heartbeatInterval
)

// ======================================================
// Node modes
// ======================================================
const (
	UNKNOWN  = "UNKNOWN"
	FOLLOWER = "FOLLOWER"
	LEADER   = "LEADER"
	ELECTION = "ELECTION"
)

const (
	NO_RESPONSE = "noResponse"
	HEALTHY     = "healthy"
	DEAD        = "sick"
)

// ======================================================
// This will be used in a map to keep the node information
// ======================================================
type nodeHost struct {
	Hostname  string `json:"hostname"` // hostname of the node
	ProcessId string `json:"processId"`
	UniqueId  string `json:"uniqueId"`
}

type genericMessage struct {
	RequestId string   `json:"requestId"` // eg "heartbeat", "nomination", "vote"
	Data      string   `json:"data"`      // optional data
	Sender    nodeHost `json:"sender"`
	Timestamp string   `json:"timestamp"`
}

// ======================================================
// This is for keeping track of the nodes status by the leader
// ======================================================
// type nodesMode struct {
// 	genericMessage
// 	Mode string `json: "mode"` // is it leader, follower or election
// }
