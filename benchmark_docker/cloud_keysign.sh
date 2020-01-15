#!/bin/bash
echo $1
curl -X POST \
  http://157.245.202.36:8080/keysign \
  -H 'Content-Type: application/json' \
  -H 'Postman-Token: 708bf50a-504a-4c52-a04d-91c603c2b644' \
  -H 'cache-control: no-cache' \
  -d '{"pool_pub_key":"'$1'","message":"YWE="}'& >/dev/null  2>&1

  sleep 1

  curl -X POST \
  http://157.245.202.163:8080/keysign \
  -H 'Content-Type: application/json' \
  -H 'Postman-Token: 708bf50a-504a-4c52-a04d-91c603c2b644' \
  -H 'cache-control: no-cache' \
  -d '{"pool_pub_key":'$1',"message":"YWE="}'& >/dev/null 2>&1

  sleep 1

  curl -X POST \
  http://157.245.202.167:8080/keysign \
  -H 'Content-Type: application/json' \
  -H 'Postman-Token: 708bf50a-504a-4c52-a04d-91c603c2b644' \
  -H 'cache-control: no-cache' \
  -d '{"pool_pub_key":"'$1'","message":"YWE="}'& >/dev/null 2>&1

  sleep 1

  curl -X POST \
  http://157.245.202.145:8080/keysign \
  -H 'Content-Type: application/json' \
  -H 'Postman-Token: 708bf50a-504a-4c52-a04d-91c603c2b644' \
  -H 'cache-control: no-cache' \
  -d '{"pool_pub_key":"'$1'","message":"YWE="}'