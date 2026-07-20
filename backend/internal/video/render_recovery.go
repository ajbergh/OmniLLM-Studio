package video

import (
	"context"
)

// RecoverInterruptedRenderJobsDurable resumes queued jobs and marks only jobs
// that had already started as interrupted. Queued work therefore survives a
// normal server or desktop restart.
func (s *Service) RecoverInterruptedRenderJobsDurable() {
	jobs, err := s.renderJobs.ListActive()
	if err != nil {
		return
	}
	for _, job := range jobs {
		if job.Status == "running" {
			_ = s.renderJobs.MarkFailed(job.ID, "render interrupted while FFmpeg was running — retry the export")
			continue
		}
		renderCtx, cancel := context.WithCancel(context.Background())
		s.renderCancelsMu.Lock()
		s.renderCancels[job.ID] = cancel
		s.renderCancelsMu.Unlock()
		go func(jobID string, cancel context.CancelFunc) {
			defer func() { s.renderCancelsMu.Lock(); delete(s.renderCancels, jobID); s.renderCancelsMu.Unlock(); cancel() }()
			s.runRenderJob(renderCtx, jobID)
		}(job.ID, cancel)
	}
}
