package data

type WorkerStateEnum string

const (
	WS_Online_healthy             WorkerStateEnum = "online_healthy"
	WS_Online_shutdown_req        WorkerStateEnum = "online_shutdown_req"
	WS_Online_shutdown_hat        WorkerStateEnum = "online_shutdown_hat"
	WS_Online_shutdown_permit     WorkerStateEnum = "online_shutdown_permit"
	WS_Offline_graceful_period    WorkerStateEnum = "offline_graceful_period"
	WS_Offline_draining_candidate WorkerStateEnum = "offline_draining_candidate"
	WS_Offline_draining_hat       WorkerStateEnum = "offline_draining_hat"
	WS_Offline_draining_complete  WorkerStateEnum = "offline_draining_complete"
	WS_Offline_dead               WorkerStateEnum = "offline_dead" // this is the satet after worker is deleted, housekeeping will clean it up.
)
