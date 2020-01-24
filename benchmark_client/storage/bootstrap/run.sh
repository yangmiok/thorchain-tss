#!/bin/bash
echo "nameserver 8.8.8.8">>/etc/resolv.conf
go build ./cmd/tss/main.go;echo "YmNiMzA2ODU1NWNjMzk3NDE1OWMwMTM3MDU0NTNjN2YwMzYzZmVhZDE5NmU3NzRhOTMwOWIxN2QyZTQ0MzdkNg==" | /home/user/tss/main -home /home/user/config