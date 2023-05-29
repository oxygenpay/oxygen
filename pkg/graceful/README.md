# Graceful

> What's [graceful-shutdown](https://whatis.techtarget.com/definition/graceful-shutdown-and-hard-shutdown)

## Example:

```go
    package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/oxygenpay/oxygen/pkg/graceful"
)

func main() {
	srv := &http.Server{}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			// example of "force" shutdown
			graceful.ShutdownNow()
		}
	}()

	// you can add as many callbacks as you want. 
	// they will be shut down in descending order (from last to first)

	// add sample callback #1
	graceful.AddCallback(srv.Close)

	// add sample callback #2
	graceful.AddCallback(func() error {
		log.Println("shutting down")
		return nil
	})

	// sample custom error handler
	graceful.ExecOnError(func(err error) {
		fmt.Printf(err.Error())
	})

	// like wg.Wait(), this operation blocks current goroutine
	graceful.WaitShutdown()
}

```
