package raft

import (
	"fmt"
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
	Term          int
	Success       bool
	ConflictIndex int
	ConflictTerm  int
}

//
// AppendEntries RPC handler.
//
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// #1
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

	// candidate -> follower
	if rf.state == CANDIDATE {
		rf.toFollower()
	}

	// reset election timer
	rf.resetElectionTimer()

	rf.leaderId = args.LeaderId
	reply.Term = rf.currentTerm
	reply.ConflictIndex = -1
	reply.ConflictTerm = -1
	reply.Success = false

	// #2: follower does not have prevLogIndex in its log
	if len(rf.logs) <= args.PrevLogIndex {
		reply.ConflictIndex = len(rf.logs)
		return
	}

	// #2: follower does have prevLogIndex in its log, but the term does not match
	if rf.logs[args.PrevLogIndex].Term != args.PrevLogTerm {
		reply.ConflictTerm = rf.logs[args.PrevLogIndex].Term
		for i := args.PrevLogIndex - 1; i >= 0; i-- {
			if rf.logs[i].Term != reply.ConflictTerm {
				reply.ConflictIndex = i + 1
				break
			}
		}
		return
	}

	reply.Success = true

	// use args.Entries to update this peer's logs
	for i, entry := range args.Entries {
		logIndex := i + args.PrevLogIndex + 1
		// #3, conflict occurs, truncate peer's logs
		if logIndex < len(rf.logs) && rf.logs[logIndex].Term != entry.Term {
			rf.logs = rf.logs[:logIndex]
		}
		// #4, append new entries (if any)
		if logIndex >= len(rf.logs) {
			rf.logs = append(rf.logs, args.Entries[i:]...)
			break
		}
	}

	// #5, set commitIndex
	if args.LeaderCommit > rf.commitIndex {
		if args.LeaderCommit < len(rf.logs)-1 {
			rf.commitIndex = args.LeaderCommit
		} else {
			rf.commitIndex = len(rf.logs) - 1
		}
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
			var entries []LogEntry
			rf.mu.Lock()
			if len(rf.logs) > rf.nextIndex[peerIndex] {
				entries = append(entries, rf.logs[rf.nextIndex[peerIndex]:]...)
			}
			prevLogIndex := rf.nextIndex[peerIndex] - 1
			if prevLogIndex == len(rf.logs) {
				prevLogIndex = len(rf.logs) - 1
			}
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
						// update nextIndex and matchIndex for that peer (follower)
						if reply.Success {
							matchIndex := args.PrevLogIndex + len(args.Entries)
							rf.matchIndex[peerIndex] = matchIndex
							rf.nextIndex[peerIndex] = matchIndex + 1
							fmt.Printf("matchIndex: %v, nextIndex: %v, log len: %v\n", matchIndex, matchIndex+1, len(rf.logs))
							// find an index n (if any), to update leader's commitIndex
							for n := len(rf.logs) - 1; n > rf.commitIndex; n-- {
								if rf.logs[n].Term != rf.currentTerm {
									continue
								}
								// the entry has replicated to leader itself
								count := 1
								for i := range rf.peers {
									if i != rf.me && rf.matchIndex[i] >= n {
										count++
										if count > len(rf.peers)/2 {
											rf.commitIndex = n
											return
										}
									}
								}
							}
						} else {
							// the accelerated log backtracking optimization
							if reply.ConflictTerm == -1 {
								rf.nextIndex[peerIndex] = reply.ConflictIndex
								return
							}
							for i := len(rf.logs) - 1; i > 0; i-- {
								if rf.logs[i].Term < reply.ConflictTerm {
									break
								}
								if rf.logs[i].Term == reply.ConflictTerm {
									rf.nextIndex[peerIndex] = i + 1
									return
								}
							}
							rf.nextIndex[peerIndex] = reply.ConflictIndex
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

		// heartbeat timeout (120ms)
		time.Sleep(120 * time.Millisecond)

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
		//time.Sleep(10 * time.Millisecond)

		rf.mu.Lock()
		for rf.commitIndex > rf.lastApplied && rf.lastApplied < len(rf.logs)-1 {
			rf.lastApplied++
			msg := ApplyMsg{
				Command:      rf.logs[rf.lastApplied].Command,
				CommandIndex: rf.lastApplied,
				CommandValid: true,
			}
			rf.mu.Unlock()
			rf.applyMsgCh <- msg
			rf.mu.Lock()
			//fmt.Println(rf.me, "has committed: ", rf.commitIndex, "applied: ", rf.lastApplied)
		}
		rf.mu.Unlock()
	}
}
