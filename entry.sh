#!/bin/bash
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

cat << EOF > /opt/perspica/turnbeat.yml
---
output:
  redis:
    enabled: $REDIS
    host: $REDISHOST
    port: $REDISPORT
    key: $REDISKEY
    db: $REDISDB
  stdout:
    enabled: $STDOUT
filter:
  filters: ["$FILTERS"]
input:
  redis:
    enabled: $REDISIN
    host: $REDISINHOST
    port: $REDISINPORT
    db: $REDISINDB
    key: $REDISINKEY
    index: $REDISINKEY
    type: $REDISINTYPE
  ${TYPE}_${PORT}:
    enabled: $TCPIN
    port: $PORT
    type: "$EVENTTYPE"
EOF

exec /opt/perspica/turnbeat $DEBUGOPTS
