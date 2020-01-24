#!/bin/bash
echo $1
curl -X POST \
  http://localhost:8320/keysign \
  -H 'Content-Type: application/json' \
  -H 'Postman-Token: 708bf50a-504a-4c52-a04d-91c603c2b644' \
  -H 'cache-control: no-cache' \
  -d '{"pool_pub_key":"'$1'","message":"YWE="}'& >/dev/null  2>&1

#  sleep 1

#  curl -X POST \
#  http://localhost:8321/keysign \
#  -H 'Content-Type: application/json' \
#  -H 'Postman-Token: 708bf50a-504a-4c52-a04d-91c603c2b644' \
#  -H 'cache-control: no-cache' \
#  -d '{"pool_pub_key":'$1',"message":"YWE="}'& >/dev/null 2>&1

  sleep 1

  curl -X POST \
  http://localhost:8322/keysign \
  -H 'Content-Type: application/json' \
  -H 'Postman-Token: 708bf50a-504a-4c52-a04d-91c603c2b644' \
  -H 'cache-control: no-cache' \
  -d '{"pool_pub_key":"'$1'","message":"YWE="}'& >/dev/null 2>&1

  sleep 1

  curl -X POST \
  http://localhost:8323/keysign \
  -H 'Content-Type: application/json' \
  -H 'Postman-Token: 708bf50a-504a-4c52-a04d-91c603c2b644' \
  -H 'cache-control: no-cache' \
  -d '{"pool_pub_key":"'$1'","message":"YWE="}'
