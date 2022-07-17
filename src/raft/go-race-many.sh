#!/bin/bash
#
# All credits to Jon Gjengset at https://gist.github.com/jonhoo, this is a modification
# version of his dedicated work at https://gist.github.com/jonhoo/f686cacb4b9fe716d5aa
#
# Script for running `go test` a bunch of times, in parallel, storing the test
# output as you go, and showing a nice status output telling you how you're
# doing.
#
# Normally, you should be able to execute this script with
#
#   ./go-race-many.sh
#
# and it should do The Right Thing(tm) by default. However, it does take some
# arguments so that you can tweak it for your testing setup. To understand
# them, we should first go quickly through what exactly this script does.
#
# First, it compiles your Go program (using go test -c) to ensure that all the
# tests are run on the same codebase, and to speed up the testing. Then, it
# runs the tester some number of times. It will run some number of testers in
# parallel, and when that number of running testers has been reached, it will
# wait for the oldest one it spawned to finish before spawning another. The
# output from each test i is stored in test-$i.log and test-$i.err (STDOUT and
# STDERR respectively).
#
# The options you can specify on the command line are:
#
#   [-r | --rounds]                  how many rounds to run the tester (defaults to 100)
#   [-p | --processes]               how many tester processes to run in parallel (defaults to the number of CPUs)
#   [-t | --test-pattern]            which subset of the tests to run (default to all tests)
#   [-c | --clear-results-directory] delete all files in results directory
#   [-race]                          whether to enable go's data race detector
#
# the [-t | --test-pattern] argument is simply a regex that is passed to the tester
# under -test.run; any tests matching the regex will be run.
#
# The script is smart enough to clean up after itself if you kill it
# (in-progress tests are killed, their output is discarded, and no failure
# message is printed), and will automatically continue from where it left off
# if you kill it and then start it again.
#
# By now, you know everything that happens below.
# If you still want to read the code, go ahead.

# results output directory
dir='test-results'

# default argument values
rounds=100
processes=$(grep -c processor /proc/cpuinfo)
test=''
clear_dir=false
race=false

while [[ $# -gt 0 ]]; do
  case $1 in
  -r | --rounds)
    if [[ $2 =~ ^[1-9][0-9]*$ ]]; then
      rounds=$2
    fi
    shift 2
    ;;
  -p | --processes)
    if [[ $2 =~ ^[1-9][0-9]*$ ]]; then
      processes=$2
    fi
    shift 2
    ;;
  -t | --test-pattern)
    test=$2
    shift 2
    ;;
  -c | --clear-results-directory)
    clear_dir=true
    shift
    ;;
  -race)
    race=true
    shift
    ;;
  -h | --help)
    echo "Usage: $0 [--rounds r] [--processes p] [--test-pattern ''] [--clear-results-directory] [-race]"
    echo "Or in short form:"
    echo "Usage: $0 [-r r] [-p p] [-t ''] [-c] [-race]"
    echo ""
    echo "By default, this script runs with the form as below:"
    echo "$0 -r 100 -p #cpus -t ''"
    exit 0
    ;;
  -*)
    echo "$0: Invalid option -- '$1'"
    echo "Try '$0 -h' or '$0 --help' for more information."
    exit 1
    ;;
  *) ;;
  esac
done

# If the tests don't even build, don't bother. Also, this gives us a static
# tester binary for higher performance and higher reproducibility.
if $race; then
  cmd="go test -race -c -o tester"
else
  cmd="go test -c -o tester"
fi

if ! $cmd; then
  echo -e "\e[1;31mERROR: Build failed\e[0m"
  exit 1
fi

mkdir -p $dir

if $clear_dir; then
  rm -rf ${dir:?}/*
  echo -e "\e[1;35mAll files in ./$dir are deleted!\e[0m"
fi

echo -e "\e[1;94mBench tests config \U00bb [test pattern: '$test', rounds: $rounds, processes: $processes, enable data race detector: $race, test results directory: ./$dir]\e[0m"

# Figure out where we left off.
logs=$(find $dir -maxdepth 1 -name 'test-*.log' -type f -printf '.' | wc -c)
success=$(cat $dir/test-*.log 2>/dev/null | grep -E '^PASS$' -c)
((failed = logs - success))

# Finish checks the exit status of the tester with the given PID, updates the
# success/failed counters appropriately, and prints a pretty message.
finish() {
  if ! wait "$1"; then
    if command -v notify-send >/dev/null 2>&1 && ((failed == 0)); then
      notify-send -i weather-storm "Tests started failing" \
        "$(pwd)/$dir\n$(cat $dir/test-*.log 2>/dev/null | grep FAIL: -- | sed -e 's/.*FAIL: / - /' -e 's/ (.*)//' | sort -u)"
    fi
    ((failed += 1))
  else
    ((success += 1))
  fi

  if [ "$failed" -eq 0 ]; then
    printf "\e[1;32m"
  else
    printf "\e[1;31m"
  fi

  printf "Done %03d/%d; %d ok, %d failed\n\e[0m" \
    $((success + failed)) \
    "$rounds" \
    "$success" \
    "$failed"
}

waits=() # which tester PIDs are we waiting on?
is=()    # and which iteration does each one correspond to?

# Cleanup is called when the process is killed.
# It kills any remaining tests and removes their output files before exiting.
cleanup() {
  for pid in "${waits[@]}"; do
    kill "$pid" 2> /dev/null
    wait "$pid"
    rm -rf "$dir/test-${is[0]}.err" "$dir/test-${is[0]}.log"
    is=("${is[@]:1}")
  done
  exit 0
}
trap cleanup SIGHUP SIGINT SIGTERM

# Run remaining iterations (we may already have run some)
for i in $(seq "$((success + failed + 1))" "$rounds"); do
  # If we have already spawned the max # of testers, wait for one to
  # finish. We'll wait for the oldest one because it's easy.
  if [[ ${#waits[@]} -eq "$processes" ]]; then
    finish "${waits[0]}"
    waits=("${waits[@]:1}") # this funky syntax removes the first
    is=("${is[@]:1}")       # element from the array
  fi

  # Store this tester's iteration index
  # It's important that this happens before appending to waits(),
  # otherwise we could get an out-of-bounds in cleanup()
  is=("${is[@]}" "$i")

  # Run the tester, passing -test.run if necessary
  if [[ -z "$test" ]]; then
    ./tester -test.v 2>"$dir/test-${i}.err" >"$dir/test-${i}.log" &
    pid=$!
  else
    ./tester -test.run "$test" -test.v 2>"$dir/test-${i}.err" >"$dir/test-${i}.log" &
    pid=$!
  fi

  # Remember the tester's PID so we can wait on it later
  waits=("${waits[@]}" "$pid")
done

# Wait for remaining testers
for pid in "${waits[@]}"; do
  finish "$pid"
done

if ((failed > 0)); then
  exit 1
fi
exit 0
