#!/bin/bash
echo "nameserver 8.8.8.8">>/etc/resolv.conf
sleep 10
echo "MjQ1MDc2MmM4MjU5YjRhZjhhNmFjMmI0ZDBkNzBkOGE1ZTBmNDQ5NGI4NzM4OTYyM2E3MmI0OWMzNmE1ODZhNw==" | ./main -home /home/user/config -peer /ip4/192.168.10.3/tcp/6668/ipfs/16Uiu2HAmAWKWf5vnpiAhfdSQebTbbB3Bg35qtyG7Hr4ce23VFA8V

