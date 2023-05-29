package lock_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/oxygenpay/oxygen/internal/lock"
	"github.com/oxygenpay/oxygen/internal/test"
	"github.com/oxygenpay/oxygen/internal/util"
	"github.com/stretchr/testify/assert"
)

func TestLocker_Do(t *testing.T) {
	tc := test.NewIntegrationTest(t)
	ctx := tc.Context

	// ARRANGE
	// Given locker service
	locker := lock.New(tc.Storage)

	// And some keys
	k1 := lock.RowKey{Table: "t1", ID: 1}
	k2 := lock.RowKey{Table: "t1", ID: 2}
	k3 := lock.StringKey{Key: "k3"}

	jobsRunningByKey := map[lock.Key]*int64{
		k1: util.Ptr(int64(0)),
		k2: util.Ptr(int64(0)),
		k3: util.Ptr(int64(0)),
	}

	// Given some handful aliases for testing
	//nolint:gocritic
	simulateWork := func(k lock.Key, sleep time.Duration) {
		//t.Logf("Started job for %+v...", k)

		atomic.AddInt64(jobsRunningByKey[k], 1)
		time.Sleep(sleep)
		atomic.AddInt64(jobsRunningByKey[k], -1)

		//t.Logf("Finished job for %+v...", k)
	}

	assertNoConcurrentJobs := func(k lock.Key) {
		assert.Equal(t, int64(0), *jobsRunningByKey[k], "key %+v", k)
	}

	// ACT & ASSERT
	// Fire some concurrent jobs and check that for each
	// specific key there is only 1 job running at the same time.
	const iterations = 20
	var wg sync.WaitGroup

	run := func(k lock.Key, fn func()) {
		err := locker.Do(ctx, k, func() error {
			fn()
			return nil
		})

		assert.NoError(t, err)
		wg.Done()
	}

	wg.Add(3 * iterations)
	for i := 0; i < iterations; i++ {
		go func() {
			run(k1, func() {
				simulateWork(k1, time.Millisecond*20)
				assertNoConcurrentJobs(k1)
			})
		}()

		go func() {
			run(k2, func() {
				simulateWork(k2, time.Millisecond*30)
				assertNoConcurrentJobs(k2)
			})
		}()

		go func() {
			run(k3, func() {
				simulateWork(k3, time.Millisecond*10)
				assertNoConcurrentJobs(k3)
			})
		}()
	}

	wg.Wait()
}
