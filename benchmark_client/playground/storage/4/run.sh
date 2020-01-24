#!/bin/bash
echo "nameserver 8.8.8.8">>/etc/resolv.conf
sleep 10
PRIVKEY="OGE3M2I5NzYyZGU2ZjhhNGZlMDI0MjU1MmIzMTM3ZTFkZDQ4Y2ZkMDViNGM3MTQ4M2UxMjE1NTVlMDM2NDhlNg=="
go build ./cmd/tss/main.go;echo $PRIVKEY | ./main -home /home/user/config -peer /ip4/3.104.66.61/tcp/6668/ipfs/16Uiu2HAm1xJMFrhg9pb4AnUhrUvjGkFTvNm1rvqnmAyUorrtXcS4

