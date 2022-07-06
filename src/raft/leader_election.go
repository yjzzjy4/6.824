package raft

import (
	"math/rand"
	"time"
)

//
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

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term        int
	VoteGranted bool
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	// candidate Term < this currentTerm;
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.VoteGranted = false
		return
	}

	// receive a higher Term, change this server to follower
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.toFollower()
		//println(rf.me, "has become follower with term", rf.currentTerm)
	}

	reply.Term = rf.currentTerm
	reply.VoteGranted = false

	if rf.votedFor == nil || *rf.votedFor == args.CandidateId {
		// compare whose log is up-to-date;
		if args.LastLogTerm > rf.logs[len(rf.logs)-1].Term || args.LastLogTerm == rf.logs[len(rf.logs)-1].Term && args.LastLogIndex >= len(rf.logs)-1 {
			// grant vote and reset election timer;
			rf.votedFor = &args.CandidateId
			reply.VoteGranted = true
			//println("In term ", reply.Term, ", ", rf.me, " has voted for: ", args.CandidateId)
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

/**
 * candidate starts an election
 */
func (rf *Raft) startElection() {
	// send request vote RPC to other servers
	for i := range rf.peers {
		if i == rf.me {
			continue
		}
		// send request vote RPC in parallel
		go func(peerIndex int) {
			rf.mu.Lock()
			args := &RequestVoteArgs{
				Term:         rf.currentTerm,
				CandidateId:  rf.me,
				LastLogIndex: len(rf.logs) - 1,
				LastLogTerm:  rf.logs[len(rf.logs)-1].Term,
			}
			rf.mu.Unlock()
			reply := &RequestVoteReply{}
			ok := rf.sendRequestVote(peerIndex, args, reply)

			if ok {
				rf.mu.Lock()
				defer rf.mu.Unlock()
				// non-outdated reply
				if args.Term == rf.currentTerm {
					// higher Term discovered, step down to follower
					if reply.Term > rf.currentTerm {
						rf.currentTerm = reply.Term
						rf.toFollower()
						//println(rf.me, "has become follower with term", rf.currentTerm)
					}
					// server are still voting
					if rf.state == CANDIDATE {
						// valid vote
						if reply.VoteGranted {
							rf.voteCount++
							// server collect majority votes, wins the election
							if rf.voteCount >= len(rf.peers)/2+1 {
								rf.toLeader()
								//println("In term ", reply.Term, ", ", rf.me, " has become the leader.")
							}
							return
						}
					}
				}
			}
		}(i)
	}
}

// The startElectionTicker go routine starts a new election if this peer hasn't received
// heartsbeats recently.
func (rf *Raft) startElectionTicker() {
	for rf.killed() == false {

		// Your code here to check if a leader election should
		// be started and to randomize sleeping time using
		// time.Sleep().

		// randomized election timeout (200 - 350ms)
		timeBeforeSleep := time.Now()
		rand.Seed(time.Now().Unix() + int64(rf.me))
		time.Sleep(time.Duration(rand.Intn(151)+200) * time.Millisecond)

		rf.mu.Lock()
		// election timeout, start new election
		if rf.heartBeatTime.Before(timeBeforeSleep) && rf.state != LEADER {
			rf.toCandidate()
		}
		rf.mu.Unlock()
	}
}
