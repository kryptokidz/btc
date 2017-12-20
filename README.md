# btc

w/ thanks to [@johngibb](https://github.com/johngibb) for pretty much all of this.

[Generate a Coinbase API key](https://www.coinbase.com/settings/api). The key
should be authorized to view all your wallets and have the permissions:

- `wallet:accounts:read` 
- `wallet:transactions:read`

Save the client key and secret in your environment as `COINBASE_KEY` and
`COINBASE_SECRET`.

To run:

```sh 
$ go get github.com/jeffreylo/btc 
$ btc
```
