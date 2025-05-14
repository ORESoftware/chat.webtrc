// source file path: ./src/common/mongo/cursor-job-queue.go
package virl_mongo

import (
	"fmt"
	"sync"
)

type Job struct {
	// Define the structure of a job
	ID int
}

type JobQueue struct {
	jobs chan Job
	wg   sync.WaitGroup
}

func NewJobQueue(size int) *JobQueue {
	return &JobQueue{
		jobs: make(chan Job, size),
	}
}

func (q *JobQueue) Enqueue(job Job) {
	q.jobs <- job
}

func (q *JobQueue) Dequeue() Job {
	return <-q.jobs
}

func (q *JobQueue) StartWorkers(n int) {
	for i := 0; i < n; i++ {
		q.wg.Add(1)
		go q.worker(i)
	}
}

func (q *JobQueue) worker(id int) {
	defer q.wg.Done()
	for job := range q.jobs {
		fmt.Printf("Worker %d processing job %d\n", id, job.ID)
		// Add job processing logic here
	}
}

func (q *JobQueue) Close() {
	close(q.jobs)
	q.wg.Wait()
}
