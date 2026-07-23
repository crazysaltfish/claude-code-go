package main

import (
	"sync"
	"time"
)

// This is a simple wrapper to run the CLI.
// The actual entry point is in cmd/cli/main.go
// To run: go run ./cmd/cli
func main() {
	// See cmd/cli/main.go for the actual implementation
	println("Please run: go run ./cmd/cli")

	const numJobs = 10
	const numWorkers = 3

	jobs := make(chan int, numJobs)
	results := make(chan int, numJobs)
	var wg sync.WaitGroup

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go worker(w, jobs, results, &wg)
	}

	for j := 1; j <= numJobs; j++ {
		jobs <- j
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		println(result)
	}
}

func worker(id int, jobs <-chan int, results chan<- int, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {
		println("Worker", id, "processing job", job)
		time.Sleep(time.Second)
		results <- job * job
		println("Worker", id, "finished job", job)
	}
}
