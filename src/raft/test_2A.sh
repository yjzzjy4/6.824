#!/bin/zsh

true > test_result_2A.txt

for i in {1..50}; do
  printf 'round %s is running.\n' "$i"
  {
    printf 'round %s\n' "$i"
    go test -run 2A -race
    printf '\n'
  } >> test_result_2A.txt
  printf "round %s is done, progress: %.2f%%.\n" "$i" "1.0 * $i / 50 * 100"
done
