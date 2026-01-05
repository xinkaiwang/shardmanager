# What is Blitz?
Blitz is a load test stype/framework.
1) Blitz tests typically run as N number threads, 
2) each thread run as infinit loop to makes requests to the target service. 
3) typically each thread makes requests to the target service at a variable rate, depend on a) sleep per loop, b) actrual call elapsed time. which means this provide "soft" load and it respect server-side back-pressure.


# What is HelloBlitz?
Its the blitz style load test driver for hellosvc

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
