package agent

import (
	"hash/fnv"
	"sync"
)

// Fixed-size stripes coordinate registration/report ingestion with deletion
// without retaining an unbounded lock entry for every retired UUID.
var clientLifecycleLocks [64]sync.Mutex

func LockClientLifecycle(uuid string) func() {
	h := fnv.New32a()
	_, _ = h.Write([]byte(uuid))
	lock := &clientLifecycleLocks[h.Sum32()%uint32(len(clientLifecycleLocks))]
	lock.Lock()
	return lock.Unlock
}
