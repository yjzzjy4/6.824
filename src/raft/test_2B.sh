#!/bin/zsh

true > test_results_2B.txt

for i in {1..500}; do
  printf 'round %s is running.\n' "$i"
  {
    printf 'round %s\n' "$i"
    go test -run 2B -race
    printf '\n'
  } >> test_results_2B.txt
  printf "round %s is done, progress: %.2f%%.\n" "$i" "1.0 * $i / 500 * 100"
done
