package raft

import (
	"time"
)

type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []LogEntry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term    int
	Success bool
}

//
// AppendEntries RPC handler.
//
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	// received a higher Term, change this server to follower
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.toFollower()
	}

	// TODO logs don't match with leader

	rf.leaderId = args.LeaderId
	reply.Term = rf.currentTerm
	reply.Success = false

	// candidate -> follower
	if rf.state == CANDIDATE {
		rf.toFollower()
	}

	if rf.state == FOLLOWER {
		// reset election timer
		rf.resetElectionTimer()
	}
}

//
// to send a AppendEntries RPC to a server.
//
func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) startAppendEntries() {
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		// send append entries RPC in parallel
		go func(peerIndex int) {
			rf.mu.Lock()
			args := &AppendEntriesArgs{
				Term:         rf.currentTerm,
				LeaderId:     rf.me,
				PrevLogIndex: 0,
				PrevLogTerm:  0,
				Entries:      rf.logs,
				LeaderCommit: rf.commitIndex,
			}
			rf.mu.Unlock()
			reply := &AppendEntriesReply{}
			ok := rf.sendAppendEntries(peerIndex, args, reply)
			if ok {
				rf.mu.Lock()
				defer rf.mu.Unlock()
				// valid reply (non-outdated)
				if args.Term == rf.currentTerm {
					// higher Term discovered, step down to follower
					if reply.Term > rf.currentTerm {
						rf.currentTerm = reply.Term
						rf.toFollower()
					}
					// server remains being leader
					if rf.state == LEADER {
						// todo: apply command or modify nextIndex, matchIndex, etc
					}
				}
			}
		}(i)
	}
}

// The startElectionTicker go routine starts a new election if this peer hasn't received
// heartsbeats recently.

func (rf *Raft) appendEntriesTicker() {
	for rf.killed() == false {

		// heartbeat timeout (100ms)
		time.Sleep(100 * time.Millisecond)

		rf.mu.Lock()
		if rf.state == LEADER {
			rf.startAppendEntries()
		}
		rf.mu.Unlock()
	}
}
