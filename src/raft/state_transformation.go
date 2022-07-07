package raft

type State int

const (
	FOLLOWER State = iota
	CANDIDATE
	LEADER
)

func (rf *Raft) toFollower() {
	rf.state = FOLLOWER
	rf.votedFor = nil
	rf.voteCount = 0
}

func (rf *Raft) toCandidate() {
	rf.state = CANDIDATE
	rf.currentTerm++
	rf.votedFor = &rf.me
	rf.voteCount = 1
	rf.resetElectionTimer()
	rf.startElection()
}

func (rf *Raft) toLeader() {
	rf.state = LEADER
	rf.leaderId = rf.me

	rf.nextIndex = make([]int, len(rf.peers))
	for i := range rf.peers {
		rf.nextIndex[i] = len(rf.logs)
	}

	rf.matchIndex = make([]int, len(rf.peers))
	rf.matchIndex[rf.me] = len(rf.logs) - 1
}
