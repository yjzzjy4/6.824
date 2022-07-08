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

	rf.leaderId = args.LeaderId
	reply.Term = rf.currentTerm
	reply.Success = true

	// candidate -> follower
	if rf.state == CANDIDATE {
		rf.toFollower()
	}

	if rf.state == FOLLOWER {
		// reset election timer
		rf.resetElectionTimer()
	}

	// peer's logs don't match with leader's logs
	if len(rf.logs) <= args.PrevLogIndex || rf.logs[args.PrevLogIndex].Term != args.PrevLogTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	// use args.Entries to update this peer's logs
	for i, entry := range args.Entries {
		logIndex := i + args.PrevLogIndex + 1
		// conflict occurs, truncate peer's logs
		if logIndex < len(rf.logs) && rf.logs[logIndex].Term != entry.Term {
			rf.logs = rf.logs[:logIndex]
		}
		// append new entries (if any)
		if logIndex >= len(rf.logs) {
			rf.logs = append(rf.logs, args.Entries[i:]...)
			break
		}
	}

	// set commitIndex
	if args.LeaderCommit > rf.commitIndex {
		if args.LeaderCommit < len(rf.logs)-1 {
			rf.commitIndex = args.LeaderCommit
			return
		}
		rf.commitIndex = len(rf.logs) - 1
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
			var entries []LogEntry
			if len(rf.logs) > rf.nextIndex[peerIndex] {
				entries = rf.logs[rf.nextIndex[peerIndex]:]
			}
			prevLogIndex := rf.nextIndex[peerIndex] - 1
			args := &AppendEntriesArgs{
				Term:         rf.currentTerm,
				LeaderId:     rf.me,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  rf.logs[prevLogIndex].Term,
				Entries:      entries,
				LeaderCommit: rf.commitIndex,
			}
			rf.mu.Unlock()
			reply := &AppendEntriesReply{}

			if rf.sendAppendEntries(peerIndex, args, reply) {
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
						// update nextIndex and matchIndex for that peer (follower)
						if reply.Success {
							matchIndex := args.PrevLogIndex + len(args.Entries)
							nextIndex := matchIndex + 1
							rf.matchIndex[peerIndex] = matchIndex
							rf.nextIndex[peerIndex] = nextIndex
							// find an index n (if any), to update leader's commitIndex
							for n := len(rf.logs) - 1; n > rf.commitIndex; n-- {
								if rf.logs[n].Term != rf.currentTerm {
									continue
								}
								count := 0
								for i := range rf.peers {
									if i != rf.me && rf.matchIndex[i] >= n {
										count++
										if count > len(rf.peers)/2 {
											break
										}
									}
								}
								if count > len(rf.peers)/2 {
									rf.commitIndex = n
									break
								}
							}
						} else {
							// decrement nextIndex and retry (on next heartbeat tick)
							rf.nextIndex[peerIndex]--
						}
					}
				}
			}
		}(i)
	}
}

// The appendEntriesTicker go routine starts a new election if this peer hasn't received
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

// The applyEntriesTicker go routine is for checking if there are
// new entries to be applied for each peer.

func (rf *Raft) applyEntriesTicker() {
	for rf.killed() == false {

		// pause execution, otherwise the loop would slow down the entire implementation,
		// causing a fail to the tests.
		time.Sleep(10 * time.Millisecond)

		rf.mu.Lock()
		for rf.commitIndex > rf.lastApplied && rf.lastApplied < len(rf.logs) {
			rf.lastApplied++
			rf.applyMsgCh <- ApplyMsg{
				Command:      rf.logs[rf.lastApplied].Command,
				CommandIndex: rf.lastApplied,
				CommandValid: true,
			}
		}
		rf.mu.Unlock()
	}
}
