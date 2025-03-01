# Worker life-cycle

## worker states
* online_healthy
* online_shutdown_req
* online_shutdown_hat
* online_shutdown_permit (subject to tombstone removeal)
* offline_graceful
* offline_draining_candidate
* offline_draining_hat
* offline_draining_complete (subject to tombstone removeal)

## in-comming events
* eph_showup
* eph_gone
* eph_update
* timer_expired
