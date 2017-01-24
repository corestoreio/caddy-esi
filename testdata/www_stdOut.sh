#!/usr/bin/env bash

cat << EOF
<span itemprop="dateUpdated">
    <relative-time datetime="2017-01-23T20:07:40Z">Jan 23, 2017</relative-time>
</span>
EOF

# read from stdin
STDIN=$(cat)
# now output of the JSON Request Arguments
echo $STDIN
