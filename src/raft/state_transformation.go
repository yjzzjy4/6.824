package raft

type State int

const (
	FOLLOWER State = iota
	CANDIDATE
	LEADER
)

func (rf *Raft) adoptHigherTerm(term int) {
	if term > rf.currentTerm {
		rf.state = FOLLOWER
		rf.currentTerm = term
		rf.votedFor = -1
		rf.persist()
	}
}

func (rf *Raft) toFollower() {
	rf.state = FOLLOWER
	rf.votedFor = -1
	rf.persist()
}

func (rf *Raft) toCandidate() {
	rf.state = CANDIDATE
	rf.currentTerm++
	rf.votedFor = rf.me
	rf.persist()
	rf.resetElectionTimer()
	rf.startElection()
}

func (rf *Raft) toLeader() {
	rf.state = LEADER
	rf.leaderId = rf.me
	//fmt.Printf("%d has become leader\n", rf.me)

	for i := range rf.peers {
		rf.matchIndex[i] = 0
		rf.nextIndex[i] = rf.lastLogIndex() + 1
	}
}
