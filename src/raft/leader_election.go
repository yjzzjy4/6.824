package raft

import (
	"fmt"
	"math/rand"
	"time"
)

// RequestVoteArgs
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

// RequestVoteReply
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int
	VoteGranted bool
}

//
// reset election timeout.
//
func (rf *Raft) resetElectionTimer() {
	rf.heartBeatTime = time.Now()
}

//
// RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// #1
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	// received a higher Term, change this server to follower
	if args.Term > rf.currentTerm {
		rf.toFollower()
		rf.currentTerm = args.Term
		rf.persist()
		fmt.Printf("%v, to term: %v, is leader: %v, reason: adopt higher term in RequestVote.\n", rf.me, rf.currentTerm, rf.state == LEADER)
		//rf.adoptHigherTerm(args.Term)
	}

	reply.Term = rf.currentTerm
	reply.VoteGranted = false

	// #2
	if rf.votedFor == -1 || rf.votedFor == args.CandidateId {
		// compare whose log is up-to-date;
		if args.LastLogTerm > rf.logs[len(rf.logs)-1].Term ||
			args.LastLogTerm == rf.logs[len(rf.logs)-1].Term && args.LastLogIndex >= len(rf.logs)-1 {
			// grant vote and reset election timer;
			rf.votedFor = args.CandidateId
			reply.VoteGranted = true
			rf.persist()
			rf.resetElectionTimer()
		}
	}
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

// candidate starts an election.
func (rf *Raft) startElection() {
	voteCount := 1
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		// send request vote RPC in parallel
		go func(peerIndex int, voteCount *int) {
			rf.mu.Lock()
			// candidate identity validation
			if rf.state != CANDIDATE {
				rf.mu.Unlock()
				return
			}
			args := &RequestVoteArgs{
				Term:         rf.currentTerm,
				CandidateId:  rf.me,
				LastLogIndex: len(rf.logs) - 1,
				LastLogTerm:  rf.logs[len(rf.logs)-1].Term,
			}
			rf.mu.Unlock()
			reply := &RequestVoteReply{}

			if rf.sendRequestVote(peerIndex, args, reply) {
				rf.mu.Lock()
				defer rf.mu.Unlock()
				// valid reply (non-outdated)
				if args.Term == rf.currentTerm {
					// higher Term discovered, step down to follower
					if reply.Term > rf.currentTerm {
						rf.toFollower()
						rf.currentTerm = reply.Term
						rf.persist()
						fmt.Printf("%v, to term: %v, is leader: %v, reason: adopt higher term in startElection.\n", rf.me, rf.currentTerm, rf.state == LEADER)
						//rf.adoptHigherTerm(args.Term)
					}
					// server are still voting
					if rf.state == CANDIDATE {
						// valid vote
						if reply.VoteGranted {
							*voteCount++
							// server collect majority votes, wins the election
							if *voteCount > len(rf.peers)/2 {
								rf.toLeader()
								// send heartbeats to other peers immediately!
								rf.startAppendEntries()
							}
						}
					}
				}
			}
		}(i, &voteCount)
	}
}

// The startElectionTicker go routine starts a new election if this peer hasn't received
// heartsbeats recently.
func (rf *Raft) startElectionTicker() {
	for !rf.killed() {

		// Your code here to check if a leader election should
		// be started and to randomize sleeping time using
		// time.Sleep().

		// randomized election timeout (200 - 400ms)
		timeBeforeSleep := time.Now()
		rand.Seed(time.Now().Unix() + int64(rf.me))
		time.Sleep(time.Duration(rand.Intn(201)+200) * time.Millisecond)

		rf.mu.Lock()
		// election timeout, start new election
		if rf.heartBeatTime.Before(timeBeforeSleep) && rf.state != LEADER {
			rf.toCandidate()
		}
		rf.mu.Unlock()
	}
}
