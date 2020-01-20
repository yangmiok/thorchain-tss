#!/bin/bash
echo "nameserver 8.8.8.8">>/etc/resolv.conf
sleep 10
PRIVKEY="YjkzMTZjYTVhMzA2OTRjM2NjYjNhN2ZkOTk4NDEwNGFkMDI1N2JiMjMzZGJmOWJhZjdkN2M5NmUzMTljOThkOA=="
go build ./cmd/tss/main.go;echo $PRIVKEY | ./main -home /home/user/config -peer /ip4/3.104.66.61/tcp/6668/ipfs/16Uiu2HAm1xJMFrhg9pb4AnUhrUvjGkFTvNm1rvqnmAyUorrtXcS4

