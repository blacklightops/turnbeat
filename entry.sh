#!/bin/bash -x
if [ -z ${TYPE} ]; then
    echo "Must Specifiy TYPE variable" 1>&2
    echo "Options are tcp udp syslog" 1>&2
    exit 1
fi

if [ -z ${PORT} ]; then
    echo "Must Specifiy PORT variable" 1>&2
    exit 1
fi

if [ -z ${EVENTTYPE} ]; then
    echo "Must Specifiy EVENTTYPE variable" 1>&2
    exit 1
fi

if [ -z ${FILTERS} ]; then
    echo "Must Specifiy FILTERS variable" 1>&2
    exit 1
fi

if [ -z ${REDISKEY} ]; then
  REDISKEY=turnbeat
fi

if [ -z ${REDISHOST} ]; then
  REDISHOST=redis
fi

if [ -z ${REDISPORT} ]; then
  REDISPORT=6379
fi

if [ -z ${STDOUT} ]; then
  STDOUT=false
fi

cat << EOF > /opt/perspica/turnbeat.yml
---
output:
  redis:
    enabled: false
    host: $REDISHOST
    port: $REDISPORT
    key: $REDISKEY
    db: 0
  stdout:
    enabled: $STDOUT
filter:
  filters: ["$FILTERS"]
input:
  ${TYPE}_${PORT}:
    enabled: true
    port: $PORT
    type: "$EVENTTYPE"
EOF

exec /opt/perspica/turnbeat
