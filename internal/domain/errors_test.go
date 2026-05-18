package domain

import "testing"

func TestNotifyError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *NotifyError
		want string
	}{
		{
			name: "with event ID and reason",
			err:  &NotifyError{EventID: "evt-001", Reason: "timeout"},
			want: "notification failed for event evt-001: timeout",
		},
		{
			name: "with empty reason",
			err:  &NotifyError{EventID: "evt-001", Reason: ""},
			want: "notification failed for event evt-001: ",
		},
		{
			name: "with empty event ID",
			err:  &NotifyError{EventID: "", Reason: "connection refused"},
			want: "notification failed for event : connection refused",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
