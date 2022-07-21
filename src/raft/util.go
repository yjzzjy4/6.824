package raft

import (
	"6.824/labgob"
	"bytes"
	"log"
)

// Debug Debugging
const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

//
// Encode all the states that should be persisted,
// return the encoded data for persistence.
//
func (rf *Raft) encodeState() []byte {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.currentTerm)
	e.Encode(rf.votedFor)
	e.Encode(rf.logs)
	e.Encode(rf.snapshotLastIndex)
	e.Encode(rf.snapshotLastTerm)
	return w.Bytes()
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	rf.persister.SaveRaftState(rf.encodeState())
}

//
// save Raft's persistent state and snapshot to stable storage,
// where it can later be retrieved after a crash and restart.
//
func (rf *Raft) persistWithSnapshot(snapshot []byte) {
	rf.persister.SaveStateAndSnapshot(rf.encodeState(), snapshot)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var currentTerm int
	var votedFor int
	var snapshotLastIndex int
	var snapshotLastTerm int
	var logs []LogEntry
	if d.Decode(&currentTerm) != nil ||
		d.Decode(&votedFor) != nil ||
		d.Decode(&logs) != nil ||
		d.Decode(&snapshotLastIndex) != nil ||
		d.Decode(&snapshotLastTerm) != nil {
		// decode error...
		log.Fatal("failed to read persisted data\n")
	} else {
		rf.currentTerm = currentTerm
		rf.votedFor = votedFor
		rf.logs = logs
		rf.snapshotLastIndex = snapshotLastIndex
		rf.snapshotLastTerm = snapshotLastTerm
		if snapshotLastIndex > 0 {
			rf.lastApplied = snapshotLastIndex
		}
	}
}

// Below are some useful functions for raft peer to call,
// didn't implement those util 2D, where you should modify
// your previous codes very often. So to make things simple and
// easy to understand, I eventually implemented all those tool functions.

func (rf *Raft) lastLogTerm() int {
	if len(rf.logs) == 1 {
		return rf.snapshotLastTerm
	}
	return rf.logs[len(rf.logs)-1].Term
}

func (rf *Raft) lastLogIndex() int {
	return rf.snapshotLastIndex + len(rf.logs) - 1
}

func (rf *Raft) actualIndex(index int) int {
	//fmt.Printf("server: %d, index: %d, snapshotLastIndex: %d\n", rf.me, index, rf.snapshotLastIndex)
	return index - rf.snapshotLastIndex
}

func (rf *Raft) logAt(index int) LogEntry {
	return rf.logs[rf.actualIndex(index)]
}

func (rf *Raft) termAt(index int) int {
	i := rf.actualIndex(index)
	if i == 0 {
		return rf.snapshotLastTerm
	}
	return rf.logs[i].Term
}

// log slice operation: [begin, end of the logs].
// begin: int, included
func (rf *Raft) logsFrom(begin int) []LogEntry {
	return rf.logs[rf.actualIndex(begin):]
}

// log slice operation: [begin of the logs, end].
// end: int, included
func (rf *Raft) logsTo(end int) []LogEntry {
	return rf.logs[:rf.actualIndex(end)+1]
}

// log slice operation: [begin, end]
// begin: int, included
// end: int, included
func (rf *Raft) logsBetween(begin, end int) []LogEntry {
	return rf.logs[rf.actualIndex(begin) : rf.actualIndex(end)+1]
}
