TSS Recovery
============

This tool is intended to generate the private key of a TSS pubkey, by
combining the secrets between each TSS member.

If you pass `export` and `password` it will generate a binance keystore file
If some of your key share file is encrypted, you need to pass the node private key with ',' to
separate each private key in the order of your key share files, if a key share is not encrypted, leave
it blank.

```-encryption-keys A,,C``` means the first key share and the last key share file are encrypted.
```-encryption-keys A,B,C``` means all key share files are encrypted.

```
tss-recovery -export <file path> -password <password> -n <num of participants
3 in a 3of4> -encryption-keys A,B,C
localstate-thorpub1addwnpepq22asyxl5fmq5klvsufrx56u78capnsgk84y0v8lqf0exjfgfldxqdhurgq.json
...
```
