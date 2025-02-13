# What is HelloBlits?
Its the load test driver for hellosvc

## How to build?
```
make
```

## How to run locally?
```
export BLITZ_THREAD_COUNT=3
export BLITZ_LOOP_SLEEP_MS=0
export BLITZ_TARGET_URL=localhost:8080/api/ping
export LOG_LEVEL=debug
./bin/helloblitz
```

