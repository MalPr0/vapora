#!/usr/bin/env bash
# A room must not outlive the conversation in it. Needs a built ./vapora beside
# this script's working directory; not part of `go test` because it drives real
# processes and real timers.
set -u
VAPORA=${VAPORA:-./vapora}
work=$(mktemp -d)
trap 'pkill -f "vapora room .*port 45" 2>/dev/null; rm -rf "$work"' EXIT

count() { pgrep -f "vapora room .*port $1" | wc -l | tr -d ' '; }
fail() { echo "FAIL: $1"; exit 1; }

start() { # fifo port log [extra...]
  mkfifo "$work/$1"
  # shellcheck disable=SC2086
  ( tail -f "$work/$1" ) | $VAPORA room -plain -port "$2" -advertise "127.0.0.1:$2" ${4:-} ${5:-} > "$work/$3" 2>&1 &
}
invite_of() { grep -o 'vapora room 127[^ ]*' "$work/$1" | head -1 | sed 's/vapora room //'; }

echo "a room that empties must close"
start a.in 45101 a.log; sleep 3
inv=$(invite_of a.log)
start b.in 45102 b.log "$inv"; sleep 5
echo "!exit" > "$work/b.in"; sleep 5
[ "$(count 45101)" = "0" ] || fail "the room stayed open with nobody in it"

echo "a room with somebody left in it must stay"
start c.in 45201 c.log; sleep 3
inv=$(invite_of c.log)
start d.in 45202 d.log "$inv"
start e.in 45203 e.log "$inv"; sleep 8
echo "!exit" > "$work/d.in"; sleep 5
[ "$(count 45201)" = "1" ] || fail "the room closed while somebody was still there"

echo "-standalone must stay on its own"
start f.in 45301 f.log "" -standalone; sleep 3
inv=$(invite_of f.log)
start g.in 45302 g.log "$inv"; sleep 5
echo "!exit" > "$work/g.in"; sleep 7
[ "$(count 45301)" = "1" ] || fail "-standalone closed anyway"

echo "ALL QUORUM CHECKS PASSED"
