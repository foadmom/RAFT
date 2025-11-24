package raft

const (
	Unknown  string = "unknown"
	Follower string = "follower"
	Leader   string = "leader"
	Election string = "election"
)

// ===========================================================================
// This will be used in a map to keep the
// ===========================================================================
type nodeHost struct {
	ProcessId string `json: "processId"`
	IP        string `json: "ip"`      // IP address
	Hostname  string `json "hostname"` // hostname of the node
}

type nodesStatus struct {
	Id     string   `json: "id"`
	node   nodeHost `json: "node"`
	Status string   `json: "status"` // live or not responding"
	Mode   string   `json: "mode"`   // is it leader, follower or election
}

type request struct {
	RequestId string   `json:"requestId"` // optional Id, like unique trans ID
	Request   string   `json: "request"`
	Sender    nodeHost `json: "sender"`
}

type response struct {
	RequestId string   `json:"requestId"` // optional Id, like unique trans ID
	Data      string   `json: "data"`     //    string   `json: "request"`
	Sender    nodeHost `json: "sender"`
}
