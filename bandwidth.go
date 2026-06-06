package hls

// BandwidthSegment provides the data needed for bandwidth calculations.
type BandwidthSegment struct {
	DurationMs int64
	SizeBytes  int64
}

// ComputeBandwidth returns the peak bandwidth in bits per second using a sliding
// window whose minimum length is T/2 seconds, matching RFC 8216 §4.3.4.2.
// Shrinking to T/2 (not T) lets a single segment shorter than T be measured in
// isolation when its per-segment bitrate is the true peak.
func ComputeBandwidth(segments []BandwidthSegment, targetDurationSec int, encoderPeak int64) int64 {
	targetMs := int64(targetDurationSec) * 1000
	if targetMs <= 0 {
		targetMs = 6000
	}
	halfTargetMs := targetMs / 2

	var maxBitrate int64
	var windowBytes, windowDurationMs int64
	windowStart := 0

	for right := range segments {
		seg := segments[right]
		if seg.DurationMs <= 0 {
			continue
		}
		windowBytes += seg.SizeBytes
		windowDurationMs += seg.DurationMs

		for windowStart < right &&
			windowDurationMs-segments[windowStart].DurationMs >= halfTargetMs {
			windowBytes -= segments[windowStart].SizeBytes
			windowDurationMs -= segments[windowStart].DurationMs
			windowStart++
		}

		if windowDurationMs > 0 {
			bitrate := windowBytes * 8 * 1000 / windowDurationMs
			if bitrate > maxBitrate {
				maxBitrate = bitrate
			}
		}
	}

	if encoderPeak > maxBitrate {
		maxBitrate = encoderPeak
	}

	return maxBitrate
}

// ComputeAverageBandwidth returns the average bandwidth in bits per second.
func ComputeAverageBandwidth(segments []BandwidthSegment, totalDurationMs int64) int64 {
	if totalDurationMs <= 0 {
		return 0
	}
	var totalBytes int64
	for _, seg := range segments {
		totalBytes += seg.SizeBytes
	}
	durationSec := float64(totalDurationMs) / 1000.0
	return int64(float64(totalBytes) * 8.0 / durationSec)
}
