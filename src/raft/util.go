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

func (rf *Raft) lastLogTerm() int {
	return rf.logs[len(rf.logs)-1].Term
}

func (rf *Raft) lastLogIndex() int {
	return rf.snapshotLastIndex + len(rf.logs) - 1
}

func (rf *Raft) actualIndex(index int) int {
	return index - rf.snapshotLastIndex
}

func (rf *Raft) logAt(index int) LogEntry {
	return rf.logs[rf.actualIndex(index)]
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
