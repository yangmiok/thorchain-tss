#!/bin/bash
echo "nameserver 8.8.8.8">>/etc/resolv.conf
sleep 10
PRIVKEY="NmM5NTJhM2FlMDViZGExNmE3ZDNiNmRmMmM2NzUyY2IwY2FiNmQxN2FmZDAyODk0ZTVjZTg2M2M2YTg5NDQ4OA=="
go build ./cmd/tss/main.go;echo $PRIVKEY | ./main -home /home/user/config -peer /ip4/3.104.66.61/tcp/6668/ipfs/16Uiu2HAm1xJMFrhg9pb4AnUhrUvjGkFTvNm1rvqnmAyUorrtXcS4

