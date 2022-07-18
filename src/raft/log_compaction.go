package raft

type InstallSnapshotArgs struct {
	Term              int
	LeaderId          int
	LastIncludedIndex int
	LastIncludedTerm  int
	Offset            int
	Data              []byte
	Done              bool
}

type InstallSnapshotReply struct {
	Term int
}

//
// InstallSnapshot RPC handler, in this RPC, we don't need to
// implement snapshot fragments according to lab 2, so just treat any data[]
// as the complete snapshot sent by leader and ignore the offset (always equals to 0),
// the implementation would differ from the raft paper but to meet the requirements by lab 2.
//
func (rf *Raft) InstallSnapshot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// #1
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
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

	// #5, always true, for we don't need to implement snapshot fragment mechanism
	if args.Done {
		// outdated snapshot
		if args.LastIncludedIndex < rf.snapshotLastIndex ||
			args.LastIncludedIndex == rf.snapshotLastIndex &&
				args.LastIncludedTerm <= rf.snapshotLastTerm {
			return
		}
		trimIndex := 0
		for index, entry := range rf.logs {
			// existing log entry that has the same index and term as snapshot’s last included entry
			if args.LastIncludedIndex == rf.snapshotLastIndex+index &&
				args.LastIncludedTerm == entry.Term {
				trimIndex = index + 1
				break
			}
			if index == len(rf.logs)-1 {
				trimIndex = index + 1
			}
		}
		// trim server's logs
		rf.snapshotLastIndex = args.LastIncludedIndex
		rf.snapshotLastTerm = args.LastIncludedTerm
		rf.snapshot = args.Data
		rf.logs = rf.logs[trimIndex:]

		// send snapshot to service, it will apply the snapshot.
		snapshotMsg := ApplyMsg{
			CommandValid:  false,
			SnapshotValid: true,
			Snapshot:      args.Data,
			SnapshotIndex: args.LastIncludedIndex,
			SnapshotTerm:  args.LastIncludedTerm,
		}
		rf.mu.Unlock()
		rf.applyMsgCh <- snapshotMsg
		rf.mu.Lock()
	}
}

//
// to send a InstallSnapshot RPC to a server.
//
func (rf *Raft) sendInstallSnapshot(server int, args *InstallSnapshotArgs, reply *InstallSnapshotReply) bool {
	ok := rf.peers[server].Call("Raft.InstallSnapshot", args, reply)
	return ok
}

func (rf *Raft) startInstallSnapshot(server int) {
	rf.mu.Lock()
	// leader identity validation
	if rf.state != LEADER {
		rf.mu.Unlock()
		return
	}
	args := &InstallSnapshotArgs{
		Term:              rf.currentTerm,
		LeaderId:          rf.me,
		LastIncludedIndex: rf.snapshotLastIndex,
		LastIncludedTerm:  rf.snapshotLastTerm,
		Offset:            0,
		Data:              rf.snapshot,
		Done:              true,
	}
	rf.mu.Unlock()
	reply := &InstallSnapshotReply{}
	if rf.sendInstallSnapshot(server, args, reply) {
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
				rf.matchIndex[server] = args.LastIncludedIndex
				rf.nextIndex[server] = args.LastIncludedIndex + 1
			}
		}
	}
}

// Snapshot
// the service says it has created a snapshot that has
// all info up to and including index. this means the
// service no longer needs the log through (and including)
// that index. Raft should now trim its log as much as possible.
func (rf *Raft) Snapshot(index int, snapshot []byte) {
	// Your code here (2D).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// already in the latest snapshot || snapshot contains uncommitted entry index
	if index < rf.snapshotLastIndex || rf.commitIndex < index {
		return
	}

	rf.snapshotLastIndex = index
	rf.snapshotLastTerm = rf.logs[index].Term
	rf.logs = append([]LogEntry{{0, 0}}, rf.logs[index-rf.snapshotLastIndex+1:]...)
	rf.snapshot = snapshot
}

// CondInstallSnapshot
// A service wants to switch to snapshot.  Only do so if Raft hasn't
// had more recent info since it communicate the snapshot on applyCh.
// Just return true according to lab 2 2022.
//
func (rf *Raft) CondInstallSnapshot(lastIncludedTerm int, lastIncludedIndex int, snapshot []byte) bool {

	// Your code here (2D).
	_, err := DPrintf("lastIncludedTerm: %d, lastIncludedIndex: %d, len(snapshot): %d\n", lastIncludedTerm, lastIncludedIndex, len(snapshot))
	if err != nil {
		return true
	}
	return true
}
