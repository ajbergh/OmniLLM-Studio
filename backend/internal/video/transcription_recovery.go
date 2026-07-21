package video

import "log"

const interruptedTranscriptionMessage = "Transcription was interrupted by application restart; retry without changing the source asset."

// RecoverInterrupted marks non-terminal provider transcription jobs as failed
// after startup. The provider-neutral API does not persist a resumable remote
// operation identifier, so retry is safer than silently leaving jobs running.
func (s *VideoTranscriptionService) RecoverInterrupted() {
	if s == nil || s.transcripts == nil {
		return
	}
	count, err := s.transcripts.FailInterrupted(interruptedTranscriptionMessage)
	if err != nil {
		log.Printf("[video transcription] startup recovery failed: %v", err)
		return
	}
	if count > 0 {
		log.Printf("[video transcription] marked %d interrupted job(s) retryable", count)
	}
}
