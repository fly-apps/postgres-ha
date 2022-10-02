#!/bin/sh

if [ -f /data/.env ]; then
  export $(cat /data/.env)
fi
