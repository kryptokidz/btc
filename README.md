# btc

[Generate a Coinbase API key](https://www.coinbase.com/settings/api). The key
should be authorized to view all your wallets and have the permissions:

- `wallet:accounts:read`
- `wallet:transactions:read`

Save the client key and secret in `~/.profile`:

```
export COINBASE_KEY=
export COINBASE_SECRET=
```

To run:

```sh
$ go get github.com/kryptokidz/btc
$ btc
          Cost Basis   Amount        $      +/-        %
 -------- ---------- -------- -------- -------- --------
      BCH      $0.00     0.00     0.00     0.00    0.00%
      BTC      $0.00     0.00     0.00     0.00    0.00%
      ETH      $0.00     0.00     0.00     0.00    0.00%
      LTC      $0.00     0.00     0.00     0.00    0.00%
 -------- ---------- -------- -------- -------- --------
    Total      $0.00             $0.00     0.00    0.00%
```

## Contributors

- [@jeffreylo](https://github.com/jeffreylo)
- [@johngibb](https://github.com/johngibb)
