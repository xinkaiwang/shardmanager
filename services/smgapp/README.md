
# smgapp

## how to build
```
make
```

## how to run smgapp

### Worker 1

```bash
export LOG_LEVEL=debug
export LOG_FORMAT=simple
export API_PORT=8081
export METRICS_PORT=9091
export POD_NAME=worker-1
export COUGAR_ETCD_ENDPOINTS=localhost:2379
./bin/smgapp
```

### Worker 2

```bash
export LOG_LEVEL=debug
export LOG_FORMAT=simple
export API_PORT=8082
export METRICS_PORT=9092
export POD_NAME=worker-2
export COUGAR_ETCD_ENDPOINTS=localhost:2379
./bin/smgapp
```

## how to call
```bash
# Success: basic ping
curl localhost:8081/api/ping

# Fail: missing shard ID
curl localhost:8081/smg/ping

# Success: with shard ID
curl localhost:8081/smg/ping -H "X-Shard-Id: shard_1"
```
