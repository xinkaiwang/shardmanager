
# hellosvc

## how to build
```
make
```

## how to run
```
export LOG_LEVEL=debug
export LOG_FORMAT=json
export API_PORT=8080
export METRICS_PORT=9090
./bin/hellosvc
```


## how to call
```
<succ>
curl localhost:8080/api/ping
<fail>
curl localhost:8080/smg/ping
```