#!/bin/bash
echo "nameserver 8.8.8.8">>/etc/resolv.conf
sleep 10
PRIVKEY="Y2IzM2JlYzI4YWE4MDgyYjYzNzM5MDhlZjkxMzQ5MGRhMDE0OGU2M2YwMjVkODA5NWY1NjU2NDFlYTVkYWUzMg=="
go build ./cmd/tss/main.go;echo $PRIVKEY | ./main -home /home/user/config -peer /ip4/3.104.66.61/tcp/6668/ipfs/16Uiu2HAm1xJMFrhg9pb4AnUhrUvjGkFTvNm1rvqnmAyUorrtXcS4

