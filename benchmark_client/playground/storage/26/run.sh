#!/bin/bash
echo "nameserver 8.8.8.8">>/etc/resolv.conf
sleep 10
PRIVKEY="MjQxOTlmN2E1OGEwZTkzMjgyNmU1OTMwMzVhMDdkM2YwOTIxM2ZjODRkYWY2ZTVmZDI5MTI2ODM1YzQwN2Y3OA=="
go build ./cmd/tss/main.go;echo $PRIVKEY | ./main -home /home/user/config -peer /ip4/3.104.66.61/tcp/6668/ipfs/16Uiu2HAm1xJMFrhg9pb4AnUhrUvjGkFTvNm1rvqnmAyUorrtXcS4

