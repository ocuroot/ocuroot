package work

import "context"

func NewWorker(ctx context.Context, cfg WorkerConfig) (*Worker, error) {
	return &Worker{}, nil
}

type WorkerConfig struct {
}

type Worker struct {
}
