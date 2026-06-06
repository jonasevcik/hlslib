package hls

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeBandwidth(t *testing.T) {
	tests := []struct {
		name              string
		segments          []BandwidthSegment
		targetDurationSec int
		encoderPeak       int64
		want              int64
	}{
		{
			name:        "single segment",
			segments:    []BandwidthSegment{{SizeBytes: 524288, DurationMs: 2000}},
			encoderPeak: 0,
			want:        2097152,
		},
		{
			name: "multiple segments, uses max",
			segments: []BandwidthSegment{
				{SizeBytes: 524288, DurationMs: 2000},
				{SizeBytes: 1048576, DurationMs: 2000},
			},
			targetDurationSec: 2,
			encoderPeak:       0,
			want:              4194304,
		},
		{
			name:        "encoder peak as fallback",
			segments:    []BandwidthSegment{{SizeBytes: 524288, DurationMs: 2000}},
			encoderPeak: 5000000,
			want:        5000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeBandwidth(tt.segments, tt.targetDurationSec, tt.encoderPeak)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestComputeBandwidthSubTargetPeak verifies that a segment whose duration falls
// between T/2 and T seconds is measured in isolation (RFC 8216 §4.3.4.2).
func TestComputeBandwidthSubTargetPeak(t *testing.T) {
	highBytes := int64(3613 * 1000 * 1583 / 8000)
	segs := []BandwidthSegment{
		{SizeBytes: 760, DurationMs: 42},
		{SizeBytes: 688, DurationMs: 42},
		{SizeBytes: 896, DurationMs: 42},
		{SizeBytes: 1663, DurationMs: 42},
		{SizeBytes: 2008, DurationMs: 42},
		{SizeBytes: highBytes, DurationMs: 1583},
	}
	got := ComputeBandwidth(segs, 2, 0)
	perSeg := highBytes * 8 * 1000 / 1583
	assert.InDelta(t, float64(perSeg), float64(got), float64(perSeg)*0.01)
}

// TestComputeBandwidthShortTailNotInflating verifies that a very short tail segment
// does not inflate BANDWIDTH by being merged with the preceding segment.
func TestComputeBandwidthShortTailNotInflating(t *testing.T) {
	segs := []BandwidthSegment{
		{SizeBytes: 2_250_000, DurationMs: 6000},
		{SizeBytes: 2_250_000, DurationMs: 6000},
		{SizeBytes: 67_500, DurationMs: 150},
	}
	got := ComputeBandwidth(segs, 6, 0)
	assert.Less(t, got, int64(3_200_000))
	assert.Greater(t, got, int64(2_900_000))
}

func TestComputeBandwidthZeroTargetDuration(t *testing.T) {
	segs := []BandwidthSegment{{SizeBytes: 524288, DurationMs: 2000}}
	got := ComputeBandwidth(segs, 0, 0)
	assert.Equal(t, int64(2_097_152), got)
}

func TestComputeAverageBandwidth(t *testing.T) {
	tests := []struct {
		name            string
		segments        []BandwidthSegment
		totalDurationMs int64
		want            int64
	}{
		{
			name:            "single segment",
			segments:        []BandwidthSegment{{SizeBytes: 524288, DurationMs: 2000}},
			totalDurationMs: 2000,
			want:            2097152,
		},
		{
			name: "multiple segments",
			segments: []BandwidthSegment{
				{SizeBytes: 524288, DurationMs: 2000},
				{SizeBytes: 524288, DurationMs: 2000},
				{SizeBytes: 524288, DurationMs: 2000},
			},
			totalDurationMs: 6000,
			want:            2097152,
		},
		{
			name:            "zero duration",
			segments:        []BandwidthSegment{{SizeBytes: 524288}},
			totalDurationMs: 0,
			want:            0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ComputeAverageBandwidth(tt.segments, tt.totalDurationMs)
			assert.Equal(t, tt.want, got)
		})
	}
}
