package raft

import "log"

// Debug Debugging
const Debug = false

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug {
		log.Printf(format, a...)
	}
	return
}

// Below are some useful functions for raft peer to call,
// didn't implement those util 2D, where you should modify
// your previous codes very often. So to make things simple and
// easy to understand, I eventually implemented all those tool functions.

func (rf *Raft) lastEntryTerm() int {
	return rf.logs[len(rf.logs)-1].Term
}

func (rf *Raft) lastEntryIndex() int {
	return rf.snapshotLastIndex + len(rf.logs) - 1
}
