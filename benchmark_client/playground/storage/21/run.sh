#!/bin/bash
echo "nameserver 8.8.8.8">>/etc/resolv.conf
sleep 10
PRIVKEY="MTE5MWYyMjU1MjQ3Njk2ZjgzYTU1MzM1OGI4M2ExN2YyZmQwNTRhODQ5ZThmYmQxN2IzMGE0YTFmMWQwMzk0Yg=="
go build ./cmd/tss/main.go;echo $PRIVKEY | ./main -home /home/user/config -peer /ip4/3.104.66.61/tcp/6668/ipfs/16Uiu2HAm1xJMFrhg9pb4AnUhrUvjGkFTvNm1rvqnmAyUorrtXcS4

