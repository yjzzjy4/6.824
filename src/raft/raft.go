package raft

//
// this is an outline of the API that raft must expose to
// the service (or test-results). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or test-results)
//   in the same server.
//
import (
	//	"bytes"
	"sync"
	"sync/atomic"
	"time"

	//	"6.824/labgob"
	"6.824/labrpc"
)

//
// ApplyMsg
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// test-results) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in part 2D you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh, but set CommandValid to false for these
// other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int

	// For 2D:
	SnapshotValid bool
	Snapshot      []byte
	SnapshotTerm  int
	SnapshotIndex int
}

//
// LogEntry
// Struct for holding log entry.
//
type LogEntry struct {
	Term    int
	Command interface{}
}

//
// Raft
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	// persisted states on all servers
	currentTerm int        // latest Term server has seen
	votedFor    int        // CandidateId that received vote in current
	logs        []LogEntry // log entries

	// volatile states on all servers
	commitIndex int // index of the highest log entry known to be committed
	lastApplied int // index of the highest log entry applied to state machine

	// volatile states on leaders
	nextIndex  []int // index of the next log entry to send to each server
	matchIndex []int // index of the highest log entry known to be replicated on server

	// log compaction
	snapshot          []byte // snapshot
	snapshotLastIndex int    // last log entry index in the latest snapshot
	snapshotLastTerm  int    // last log entry term in the latest snapshot

	// other necessary states
	state         State         // raft state
	leaderId      int           // used by follower for redirecting client's request to leader
	heartBeatTime time.Time     // last heartbeat time
	applyMsgCh    chan ApplyMsg // to inform the service (or tester) whether there are newly committed entries in this peer
	applyCond     *sync.Cond    // condition to trigger apply log entry event
}

//
// GetState
// return currentTerm and whether this server
// believes it is the leader.
//
func (rf *Raft) GetState() (int, bool) {
	// Your code here (2A).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.currentTerm, rf.state == LEADER
}

//
// Start
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	// Your code here (2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if rf.state != LEADER {
		return -1, rf.currentTerm, false
	}

	// append entry to leader's logs, will be syncing during next heartbeat
	rf.logs = append(rf.logs, LogEntry{Command: command, Term: rf.currentTerm})

	rf.persist()

	return rf.lastLogIndex(), rf.currentTerm, true
}

//
// Kill
// the test-results doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
//
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

//
// Make
// the service or test-results wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// test-results or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).
	rf.leaderId = -1
	rf.currentTerm = 0
	rf.commitIndex = 0
	rf.lastApplied = 0
	rf.logs = append(rf.logs, LogEntry{0, 0})
	rf.nextIndex = make([]int, len(rf.peers))
	rf.matchIndex = make([]int, len(rf.peers))
	rf.snapshot = make([]byte, 0)
	rf.snapshotLastIndex = 0
	rf.snapshotLastTerm = 0
	rf.applyMsgCh = applyCh
	rf.applyCond = sync.NewCond(&rf.mu)
	rf.state = FOLLOWER
	rf.votedFor = -1
	rf.resetElectionTimer()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start ElectionTicker goroutine to start elections
	go rf.startElectionTicker()

	// start appendEntriesTicker goroutine to start append entries / send heartbeats
	go rf.appendEntriesTicker()

	// start applier goroutine to apply entries to each peer's state machine
	go rf.applier()

	return rf
}
