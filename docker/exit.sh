#!/bin/bash
curl -s -X POST --unix-socket /tmp/daemon.socket -H "Content-Type: application/json" -d "{\"code\": $@}" http://daemon/exit
