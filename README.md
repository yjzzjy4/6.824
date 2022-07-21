# Introduction

This document is for recording some problems or hints the author have met or concluded during the process of implementing raft ([MIT 6.824 lab2](https://pdos.csail.mit.edu/6.824/labs/lab-raft.html) experiment).

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

### hints:

0. Do not reset leader's `votedFor` field right after it becomes leader, actually, do not do that ever as a leader;
1. Heartbeat time should be greater than 100 ms, as the tester limits 10 heartbeats / sec;
2. Send `AppendEntries RPC` right after a leader is elected, the later part would be benefit from this behavior.

## Part 2B

### apply error

> A server applied a different command to its state machine at the same index as other servers applied, aka command apply inconsistency.

#### Cause for 'apply error'

0. Server adopted a higher term before step down to follower;
1. Before sending `AppendEntries RPC`s to others, not validating whether this server is (still) leader.

It's not easy to explain the scenario, one way to reproduce the problem is when a *partitioned leader* went back to the system, received a higher term from RPC reply and adopted it, if he adopted the term before step down to follower, and meanwhile it happens to be the time for him to send `AppendEntries RPC`s to others, it could lead to critical mistakes:

- the reconnected leader is outdated, but he doesn't think so. For he keeps the leader identity, and (probably) with the latest term (due to some server's reply, he adopted a higher term);
- the others received `AppendEntries RPC` from the outdated leader, *perhaps with higher term than last time*, they're confused, but they would accept this RPC, and **trim their logs and append the outdated leader's if conflict happens**;
- *the outdated leader wins the election*, and apply entries right after the index when he was disconnected before, this leads to an apply error: same index, different command!
- *even if the outdated leader fails the election*, the other server that trimmed their logs and appended the outdated leader's logs would still apply the wrong command!

This could happen when a leader and some of followers are disconnected, and then reconnected back to the system, those followers kick off the current leader, and rise the term of the outdated leader, the the outdated leader then starts to overwrite other followers' logs through `AppendEntries RPC`, the outdated leader didn't step down to follower until he overwrote a majority of followers' logs. Then, election begins, the outdated leader wins the election, leading to that error.

Even if the outdated leader didn't win the election, as long as he overwrote some of the followers' logs before they could apply all their original logs (the ones before being overwritten) to the state machine, they could probably apply with the outdated logs later.

The last but two test case for part 2B simulates the situation as we described above, read the code from test case: **Test (2B): leader backs up quickly over incorrect follower logs**, you may need to print some logs, then you will see what's going on when this problem happens.

#### Solution for 'apply error'

According to the [Students' Guide to Raft](https://thesquareplanet.com/blog/students-guide-to-raft/#the-importance-of-details):

> For example, if you have already voted in the current term, and an incoming `RequestVote` RPC has a higher term that you, you should *first* step down and adopt their term (thereby resetting `votedFor`), and *then* handle the RPC...

- You should *first* step down and adopt a higher term.

- Validating the server's Identity (it could step down after received a reply from previous `AppendEntries RPC`) before send a new `AppendEntries RPC` to other server.

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

Clearly, in `Make` function, the server persist its states (inside `rf.toFollower()`) before it recovers from the previous persisted states, the new persisted states **overwrite** the old one even before it can be read. This leads to a serve malfunction: <u>the previous persisted states are overwritten with default states</u>, next time the server apply a log entry for the same apply index, it will be a totally different one from the previous applied.

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

The solution is relatively simple, I just adjusted the arguments used in the system, for instance, modify the *<u>heartbeat time</u>* and *<u>election timeout</u>*, make sure they differ a bit from each other. I tested two groups of arguments, as below:

- *<u>heartbeat time</u>*: 80 ms, *<u>election timeout</u>*: 200 ~ 400 ms
- *<u>heartbeat time</u>*: 120 ms, *<u>election timeout</u>*: 300 ~ 500 ms

Those arguments all passed 1,000 rounds of test, but according to the requirements by [6.824](https://pdos.csail.mit.edu/6.824/labs/lab-raft.html):

> The paper's Section 5.2 mentions election timeouts in the range of 150 to 300 milliseconds. Such a range only makes sense if the leader sends heartbeats considerably more often than once per 150 milliseconds. Because the tester limits you to 10 heartbeats per second, you will have to use an election timeout larger than the paper's 150 to 300 milliseconds, but not too large, because then you may fail to elect a leader within five seconds.

You might choose the latter group of arguments to meet the requirements.

### Part 2D

### hints:

Since we would modify our code a lot in 2D, it is highly suggested that you encapsulate some useful tool functions for raft, such as `lastLogTerm()`, `lastLogIndex()`, `termAt(index int)`, `logAt(index int)`, etc.

Then, use these functions to refactor your previous code, before you started 2D, make sure the refactored version passes the tests before.

This approach can ensure that there are no subtle problems in your code, making it easier to proceed and debug for 2D.

### failed to reach agreement

#### Cause for 'failed to reach agreement'

One situation I discovered for causing the problem is that a disconnected leader comes back up, though he is disconnected too long to come back as a leader, others still voted for him, leading to a outdated leader scenario. Usually this happens when there are bugs in your code related to leader election.

#### Solution for 'failed to reach agreement'

Later I found that this is a problem related to `lastLogTerm` and truncate logs. When a server received a snapshot from the tester, if it is not outdated, then the server accepts it and truncates its logs, here is my original implementation at first:

```go
rf.logs = append([]LogEntry{{0, 0}}, rf.logsFrom(index+1)...)
rf.snapshotLastIndex = index
rf.snapshotLastTerm = rf.termAt(index)
```

As we can see, the code above truncates logs before updating `snapshotLastTerm`, due to missing log entry at index, the `snapshotLastTerm` is always 0 (default value), or even worse, this could leads to an out of index error (because the logs are truncated, it could be not long enough).

So the correct approach is to introduce a temporary variable to save the states we need before it is overwritten, like this:

```go
lastLogTerm := rf.termAt(index)
rf.logs = append([]LogEntry{{0, 0}}, rf.logsFrom(index+1)...)
rf.snapshotLastIndex = index
rf.snapshotLastTerm = lastLogTerm
```

Or we can manage the order of statements so that it won't cause the problem:

```go
rf.snapshotLastTerm = rf.termAt(index)
rf.logs = append([]LogEntry{{0, 0}}, rf.logsFrom(index+1)...)
rf.snapshotLastIndex = index
```

Why can't we update `snapshotLastIndex` before truncating logs ? Because the `logsFrom(index)` implementation depends on it:

```go
func (rf *Raft) logsFrom(begin int) []LogEntry {
	return rf.logs[rf.actualIndex(begin-rf.snapshotLastIndex):]
}
```

Moreover, after we introduce the log compaction machanism, there will be plenty of issues like this. For example, if you want to retrieve a log entry specific index, you can't just do `rf.logs[index]`, it needs to be transformed to the "real" index, use a tool functions like below:

```go
func (rf *Raft) actualIndex(index int) int {
	return index - rf.snapshotLastIndex
}
```

The same for `lastLogTerm()` and `lastLogIndex()`:

```go
func (rf *Raft) lastLogTerm() int {
	if len(rf.logs) == 1 {
		return rf.snapshotLastTerm
	}
	return rf.logs[len(rf.logs)-1].Term
}

func (rf *Raft) lastLogIndex() int {
	return rf.snapshotLastIndex + len(rf.logs) - 1
}
```

You will have to pay extra attentions to these dependencies, the transformations between those states could affect your previous code, leading to tiny, but annoying bugs. So always be careful with your statement orders.

### apply error

#### Cause for 'apply error'
