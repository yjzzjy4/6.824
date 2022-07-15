# Introduction

This document is for recording some problems or hints the author have met or concluded during the process of implementing raft (MIT 6.824 lab2 experiment).

## Before started

- Do read the [extended Raft paper](https://pdos.csail.mit.edu/6.824/papers/raft-extended.pdf);
- Do read the [Students' Guide to Raft](https://thesquareplanet.com/blog/students-guide-to-raft/#the-importance-of-details) carefully.

## Ongoing

- Whenever encountered some problem, refer to [Students' Guide to Raft](https://thesquareplanet.com/blog/students-guide-to-raft/#the-importance-of-details) first;
- Read the tester's code for better understanding of what's going on during the tests;
- Print logs wherever could lead to a bug, e.g. on term change, around send or receive RPCs, or even inside the tester's code (do not alter the codes that performs the tests, however);
- Write down the problems you have encountered, and the solutions accordingly.

## Afterwards

- Relax, think and learn something new.

# Hints && Problems

## Part 2A

Not many tricky problems, but some hints:

0. Do not reset leader's `votedFor` field right after it becomes leader, actually, do not do that ever as a leader;
1. Heartbeat time should be greater than 100 ms, as the tester limits 10 heartbeats / sec;
2. Send `AppendEntries RPC` right after a leader is elected, the later part would be benefit from this behavior.

## Part 2B

### apply error

> A server applied a different command to its state machine at the same index as other servers applied, aka command apply inconsistency.

#### Cause for 'apply error'

0. Server adopted a higher term before step down to follower;
1. Before sending `AppendEntries RPC`s to others, not validating whether this server is (still) leader.



#### Solution for 'apply error'

According to the [Students' Guide to Raft](https://thesquareplanet.com/blog/students-guide-to-raft/#the-importance-of-details):

> For example, if you have already voted in the current term, and an incoming `RequestVote` RPC has a higher term that you, you should *first* step down and adopt their term (thereby resetting `votedFor`), and *then* handle the RPC...

- For cause 0: 

    You should *first* step down and adopt a higher term.

- For cause 1: 

    Validating the server's Identity (it could step down after received a reply from previous `AppendEntries RPC`) before send a new `AppendEntries RPC` to other server.

### Part 2C

### apply error

> Same reason as 2B.

#### Cause for 'apply error'

Note that if your leader election and log replication procedures are pretty well implemented, persistence part (2C) should occur no error, check for your previous code or subtle bugs inside the timing you call `rf.persist()`, below is a stupid mistake I made during the experiment: 

At first, I use a function to convert a server's identity to follower:

```go
func (rf *Raft) toFollower() {
	rf.state = FOLLOWER
	rf.votedFor = -1
}
```

Then in 2C, I add the `rf.persist()` statement to this function:

```go
func (rf *Raft) toFollower() {
	rf.state = FOLLOWER
	rf.votedFor = -1
    rf.persist()
}
```

But I forgot that I used this function in `Make` (this function starts and initiates a server):

```go
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {

	// other initialization code...
    
    // server starts as follower
	rf.toFollower()
    
	rf.resetElectionTimer()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start some ticker goroutines...

	return rf
}
```

Clearly, in `Make` function, the server persist its states before it recovers from the previous persisted states, the new persisted states **overwrite** the old one even before it can be read. This leads to a serve malfunction: <u>the previous persisted states are overwritten with default states</u>, next time the server apply a log entry for the same apply index, it will be a totally different one from the previous applied.

#### Solution for 'apply error'

The solution is quite simple actually, just modified the `Make` function into the one below:

```go
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {

	// other initialization code...
    
    // server starts as follower
	rf.state = FOLLOWER
	rf.votedFor = -1
    
	rf.resetElectionTimer()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	// start some ticker goroutines...

	return rf
}
```

### failed to reach agreement

The network is messed up during the 2C tests (to simulate server's crash and restart). Therefore your system could not be able to elect a leader or apply a specific log entry in time.

#### Solution for 'failed to reach agreement'

The solution is relatively simple, I just adjusted the arguments used in the system, for instance, modify the *<u>heartbeat time</u>* and *<u>election timeout</u>*, make sure they differ a bit from each other. I tested two groups of arguments, as shown below:

- *<u>heartbeat time</u>*: 80 ms, *<u>election timeout</u>*: 200 ~ 400 ms
- *<u>heartbeat time</u>*: 120 ms, *<u>election timeout</u>*: 300 ~ 500 ms

Those arguments all passed 1,000 rounds of test, but according to the requirements by [6.824](https://pdos.csail.mit.edu/6.824/labs/lab-raft.html):

> The paper's Section 5.2 mentions election timeouts in the range of 150 to 300 milliseconds. Such a range only makes sense if the leader sends heartbeats considerably more often than once per 150 milliseconds. Because the tester limits you to 10 heartbeats per second, you will have to use an election timeout larger than the paper's 150 to 300 milliseconds, but not too large, because then you may fail to elect a leader within five seconds.

You might choose the latter group of arguments to meet the requirements.