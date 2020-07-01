
```bash
make go.build.linux_amd64.provider

./linux/amd64/provider -p 9090

curl localhost:9090
```

```bash
make go.build.linux_amd64.consumer

./linux/amd64/consumer -p 5555

curl localhost:5555?p=http://localhost:9090
```




