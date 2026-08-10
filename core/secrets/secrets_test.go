package secrets

import (
	"fmt"
	"sync"
	"testing"
)

func TestCredentialStoreSupportsConcurrentReplacement(t *testing.T) {
	Replace(Credentials{MediaServer: "initial"})
	var wait sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wait.Add(1)
		go func(worker int) {
			defer wait.Done()
			for iteration := 0; iteration < 1000; iteration++ {
				Replace(Credentials{MediaServer: fmt.Sprintf("%d-%d", worker, iteration)})
				_ = Current().MediaServer
			}
		}(worker)
	}
	wait.Wait()
	if Current().MediaServer == "" {
		t.Fatal("credential store lost the installed value")
	}
}
