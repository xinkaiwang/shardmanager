# What is HelloBlits?
Its the load test driver for hellosvc

## How to build?
```
make
```

## How to run locally?
```
export BLITZ_THREAD_COUNT=1
export BLITZ_LOOP_SLEEP_MS=1000
export BLITZ_TARGET_URL=localhost:8080/api/ping
export LOG_LEVEL=debug
./bin/helloblitz
```


## Run unicornblitz locally?
```
export BLITZ_THREAD_COUNT=1
export BLITZ_LOOP_SLEEP_MS=5000
export LOG_LEVEL=debug
./bin/unicornblitz
```

## How to make calls to smgapp?
```
curl localhost:8081/api/ping
```

```
curl "localhost:8081/smg/ping?name=ho" -H "X-Shard-Id: shard_c0_00"
```
