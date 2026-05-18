package domain

import "testing"

func TestEventStatusConstants(t *testing.T) {
	tests := []struct {
		name   string
		status EventStatus
		want   string
	}{
		{
			name:   "pending status",
			status: StatusPending,
			want:   "pending",
		},
		{
			name:   "delivered status",
			status: StatusDelivered,
			want:   "delivered",
		},
		{
			name:   "failed status",
			status: StatusFailed,
			want:   "failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(tt.status)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
