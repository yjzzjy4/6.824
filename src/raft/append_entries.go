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

	// peer's logs don't match with leader's logs
	if len(rf.logs)-1 < args.PrevLogIndex || rf.logs[args.PrevLogIndex].Term != args.PrevLogTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		return
	}

	appendOffset, entryLength, logLength := 0, len(args.Entries), len(rf.logs)
	// check if entry conflict exists
	for i := args.PrevLogIndex + 1; i <= logLength; i++ {
		entryIndex := i - args.PrevLogIndex - 1
		if i == logLength {
			// entries are longer than peer's logs, without conflict
			if entryIndex < entryLength {
				appendOffset = entryIndex
				break
			}
			break
		}
		// peer's log are longer than entries, without conflict
		if entryIndex == entryLength {
			appendOffset = entryLength
			break
		}
		// conflict occurs
		if rf.logs[i].Term != args.Entries[entryIndex].Term {
			// truncate server's logs
			rf.logs = rf.logs[:i]
			appendOffset = entryIndex
			break
		}
	}

	// append new entries
	appendEntries := args.Entries[appendOffset:]
	logLengthBeforeAppend := len(rf.logs)
	if len(appendEntries) > 0 {
		rf.logs = append(rf.logs, appendEntries...)
		rf.commitIndex = len(rf.logs) - 1
	}
	for i, entry := range appendEntries {
		rf.applyMsgCh <- ApplyMsg{
			Command:      entry.Command,
			CommandIndex: i + logLengthBeforeAppend,
			CommandValid: true,
		}
	}

	// set commitIndex
	if args.LeaderCommit > rf.commitIndex {
		if args.LeaderCommit < len(rf.logs)-1 {
			rf.commitIndex = args.LeaderCommit
		} else {
			rf.commitIndex = len(rf.logs) - 1
		}
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
			if len(rf.logs) >= rf.nextIndex[peerIndex] {
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
						// update nextIndex and matchIndex for the peer (follower)
						if reply.Success {
							rf.matchIndex[peerIndex] = args.PrevLogIndex + len(args.Entries)
							rf.nextIndex[peerIndex] = rf.matchIndex[peerIndex] + 1
							// check whether to update leader's commitIndex
							offset := rf.commitIndex
							replicatedIndexCount := make([]int, len(rf.logs)-offset)
							// count for how many peers that has replicated entry whose index > leader's commitIndex
							for i := range rf.peers {
								if rf.matchIndex[i] > offset {
									replicatedIndexCount[rf.matchIndex[i]-offset]++
								}
							}
							for i := len(replicatedIndexCount) - 1; i > 0; i-- {
								// a majority of peers has replicated an entry with larger index, update commitIndex
								if replicatedIndexCount[i] > len(rf.peers)/2+1 && rf.logs[i+offset].Term == rf.currentTerm {
									for j := rf.commitIndex + 1; j <= i+offset; j++ {
										rf.applyMsgCh <- ApplyMsg{
											Command:      rf.logs[j].Command,
											CommandIndex: j,
											CommandValid: true,
										}
									}
									rf.commitIndex = i + offset
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
