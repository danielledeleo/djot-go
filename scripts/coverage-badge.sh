#!/bin/sh
# Computes total statement coverage and writes a shields.io endpoint badge to
# coverage.json. CI publishes that file to the "badges" branch, which the README
# badge reads; run it locally to see what the badge would say.
set -eu

out=${1:-coverage.json}

go test -coverprofile=cov.out ./... >/dev/null
pct=$(go tool cover -func=cov.out | awk '/^total:/ {gsub(/%/,"",$3); print $3}')
rm -f cov.out

if [ -z "$pct" ]; then
	echo "could not read a total from the coverage profile" >&2
	exit 1
fi

# Shields' own palette, coarsely: green once most statements are covered,
# warmer as it drops.
color=$(awk -v p="$pct" 'BEGIN {
	if (p >= 90) print "brightgreen";
	else if (p >= 75) print "green";
	else if (p >= 60) print "yellowgreen";
	else if (p >= 45) print "yellow";
	else if (p >= 30) print "orange";
	else print "red";
}')

cat >"$out" <<EOF
{
  "schemaVersion": 1,
  "label": "coverage",
  "message": "${pct}%",
  "color": "${color}"
}
EOF

echo "coverage ${pct}% (${color}) -> ${out}"
