package video

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
		s.launchRenderJob(job.ID)
	}
}
