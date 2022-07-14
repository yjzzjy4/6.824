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

- Relax, Think and Learn something new.

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