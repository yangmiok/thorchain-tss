#!/bin/bash
echo "nameserver 8.8.8.8">>/etc/resolv.conf
sleep 10
PRIVKEY="YmMzNDNhYzQ4ZWQyZmEzMTA1ZjhlZWM0OWQxMjIwMWRjOWExMjFjYTg4ZGRjYjIwODY2NWNkMWY4MTA0MzQ5Ng=="
go build ./cmd/tss/main.go;echo $PRIVKEY | ./main -home /home/user/config -peer /ip4/3.104.66.61/tcp/6668/ipfs/16Uiu2HAm1xJMFrhg9pb4AnUhrUvjGkFTvNm1rvqnmAyUorrtXcS4

