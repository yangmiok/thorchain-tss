#!/bin/bash
echo "nameserver 8.8.8.8">>/etc/resolv.conf
sleep 10
PRIVKEY="ZWMzNjY1NzU4MGUzY2ZiMGEyZDhjYThhZjE3MjlmYWMxOWJjZGZjMmRhNjE4MTVkYjhhNjc4MjI1NWY3NmFmZg=="
go build ./cmd/tss/main.go;echo $PRIVKEY | ./main -home /home/user/config -peer /ip4/3.104.66.61/tcp/6668/ipfs/16Uiu2HAm1xJMFrhg9pb4AnUhrUvjGkFTvNm1rvqnmAyUorrtXcS4

