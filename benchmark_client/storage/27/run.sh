#!/bin/bash
echo "nameserver 8.8.8.8">>/etc/resolv.conf
sleep 10
PRIVKEY="OWYzNzhiN2IxMGQwOTZlNTlkOTE4YjZmMzIyY2MwNGUxYjI3ZjVjMWM1MGEwY2MxMDcxM2ViZGI2YjMzOGZmYg=="
go build ./cmd/tss/main.go;echo $PRIVKEY | ./main -home /home/user/config -peer /ip4/3.104.66.61/tcp/6668/ipfs/16Uiu2HAm1xJMFrhg9pb4AnUhrUvjGkFTvNm1rvqnmAyUorrtXcS4

