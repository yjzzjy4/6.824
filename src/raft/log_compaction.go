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

	reply.Term = rf.currentTerm

	// #1
	if args.Term < rf.currentTerm {
		return
	}

	// reset election timer
	rf.resetElectionTimer()

	// #2, create new snapshot file
	if args.Offset == 0 {
		// how to proceed?
	}
}

//
// to send a InstallSnapshot RPC to a server.
//
func (rf *Raft) sendInstallSnapshot(server int, args *InstallSnapshotArgs, reply *InstallSnapshotReply) bool {
	ok := rf.peers[server].Call("Raft.InstallSnapshot", args, reply)
	return ok
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

	rf.lastSnapshotIndex = index
	rf.lastSnapshotTerm = rf.logs[index].Term
	rf.logs = rf.logs[index+1:]

	// what to do with the argument: snapshot?
}

// CondInstallSnapshot
// A service wants to switch to snapshot.  Only do so if Raft hasn't
// have more recent info since it communicate the snapshot on applyCh.
//
func (rf *Raft) CondInstallSnapshot(lastIncludedTerm int, lastIncludedIndex int, snapshot []byte) bool {

	// Your code here (2D).
	_, err := DPrintf("lastIncludedTerm: %d, lastIncludedIndex: %d, len(snapshot): %d\n", lastIncludedTerm, lastIncludedIndex, len(snapshot))
	if err != nil {
		return true
	}
	return true
}
