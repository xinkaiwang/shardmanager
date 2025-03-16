module github.com/xinkaiwang/shardmanager/services/unicorn

go 1.21

require github.com/xinkaiwang/shardmanager/libs/xklib v0.0.0

require (
	github.com/sirupsen/logrus v1.9.3 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/sys v0.0.0-20220715151400-c0bba94af5f8 // indirect
)

replace github.com/xinkaiwang/shardmanager/libs/xklib => ../../libs/xklib
