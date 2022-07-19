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
		rf.toFollower()
		rf.currentTerm = args.Term
		rf.persist()
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
	if rf.lastLogIndex() < args.PrevLogIndex {
		reply.ConflictIndex = rf.lastLogIndex() + 1
		return
	}

	// #2: follower does have prevLogIndex in its log, but the term does not match
	if rf.logAt(args.PrevLogIndex).Term != args.PrevLogTerm {
		reply.ConflictTerm = rf.logAt(args.PrevLogIndex).Term
		for i := args.PrevLogIndex - 1; i >= rf.snapshotLastIndex; i-- {
			if rf.logAt(i).Term != reply.ConflictTerm {
				reply.ConflictIndex = i + 1
				break
			}
		}
		return
	}

	// snapshot already contains (partial) logs from this RPC.
	if args.PrevLogIndex < rf.snapshotLastIndex {
		reply.ConflictIndex = rf.snapshotLastIndex
		return
	}

	reply.Success = true

	// use args.Entries to update this peer's logs
	for i, entry := range args.Entries {
		entryIndex := i + args.PrevLogIndex + 1
		// #3, conflict occurs, truncate peer's logs
		if entryIndex <= rf.lastLogIndex() && rf.logAt(entryIndex).Term != entry.Term {
			rf.logs = rf.logsTo(entryIndex - 1)
			rf.persist()
		}
		// #4, append new entries (if any)
		if entryIndex > rf.lastLogIndex() {
			rf.logs = append(rf.logs, args.Entries[i:]...)
			rf.persist()
			break
		}
	}

	// #5, set commitIndex
	if args.LeaderCommit > rf.commitIndex {
		if args.LeaderCommit < rf.lastLogIndex() {
			rf.commitIndex = args.LeaderCommit
		} else {
			rf.commitIndex = rf.lastLogIndex()
		}
		rf.apply()
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
			// leader identity validation && nextIndex validation
			if rf.state != LEADER || rf.nextIndex[peerIndex] <= rf.snapshotLastIndex {
				rf.mu.Unlock()
				return
			}
			var entries []LogEntry
			if rf.lastLogIndex() >= rf.nextIndex[peerIndex] {
				entries = append(entries, rf.logsFrom(rf.nextIndex[peerIndex])...)
			}
			nextIndex := rf.nextIndex[peerIndex]
			if nextIndex > rf.lastLogIndex()+1 {
				nextIndex = rf.lastLogIndex() + 1
			}
			prevLogIndex := nextIndex - 1
			args := &AppendEntriesArgs{
				Term:         rf.currentTerm,
				LeaderId:     rf.me,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  rf.logAt(prevLogIndex).Term,
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
						rf.toFollower()
						rf.currentTerm = reply.Term
						rf.persist()
					}
					// server remains being leader
					if rf.state == LEADER {
						// update nextIndex and matchIndex for that peer (follower)
						if reply.Success {
							matchIndex := args.PrevLogIndex + len(args.Entries)
							rf.matchIndex[peerIndex] = matchIndex
							rf.nextIndex[peerIndex] = matchIndex + 1
							// find an index n (if any), to update leader's commitIndex
							for n := rf.lastLogIndex(); n > rf.commitIndex && n > rf.snapshotLastIndex; n-- {
								if rf.logAt(n).Term != rf.currentTerm {
									continue
								}
								// the entry has replicated to leader itself
								count := 1
								for i := range rf.peers {
									if i != rf.me && rf.matchIndex[i] >= n {
										count++
										if count > len(rf.peers)/2 {
											rf.commitIndex = n
											rf.apply()
											return
										}
									}
								}
							}
						} else {
							// the accelerated log backtracking optimization
							if reply.ConflictTerm == -1 {
								rf.nextIndex[peerIndex] = reply.ConflictIndex
							} else {
								foundNextIndex := false
								for i := rf.lastLogIndex(); i > rf.snapshotLastIndex; i-- {
									if rf.logAt(i).Term < reply.ConflictTerm {
										break
									}
									if rf.logAt(i).Term == reply.ConflictTerm {
										rf.nextIndex[peerIndex] = i + 1
										foundNextIndex = true
										break
									}
								}
								if !foundNextIndex {
									rf.nextIndex[peerIndex] = reply.ConflictIndex
								}
							}
							// leader sends its snapshot to a stale follower
							if rf.nextIndex[peerIndex] <= rf.snapshotLastIndex {
								go rf.startInstallSnapshot(peerIndex)
							}
						}
					}
				}
			}
		}(i)
	}
}

// The appendEntriesTicker go routine send new entries / heartbeat
// to follower periodically.
func (rf *Raft) appendEntriesTicker() {
	for !rf.killed() {

		// heartbeat timeout (120ms)
		time.Sleep(120 * time.Millisecond)

		rf.mu.Lock()
		if rf.state == LEADER {
			rf.startAppendEntries()
		}
		rf.mu.Unlock()
	}
}

// trigger an entry apply.
func (rf *Raft) apply() {
	rf.applyCond.Broadcast()
}

// apply an entry to state machine.
func (rf *Raft) applier() {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	for !rf.killed() {
		// all server rule 1
		if rf.commitIndex > rf.lastApplied && rf.lastLogIndex() > rf.lastApplied {
			rf.lastApplied++
			applyMsg := ApplyMsg{
				CommandValid: true,
				Command:      rf.logAt(rf.lastApplied).Command,
				CommandIndex: rf.lastApplied,
			}
			rf.mu.Unlock()
			rf.applyMsgCh <- applyMsg
			rf.mu.Lock()
		} else {
			rf.applyCond.Wait()
		}
	}
}
