package models

import "testing"

func TestPayment_IsTerminal(t *testing.T) {
	tests := []struct {
		status PaymentStatus
		want   bool
	}{
		{StatusInitiated, false},
		{StatusPending3DS, false},
		{StatusConfirmed, false},
		{StatusCompleted, true},
		{StatusFailed, true},
		{StatusRefunded, true},
	}

	for _, tt := range tests {
		p := &Payment{Status: tt.status}
		if got := p.IsTerminal(); got != tt.want {
			t.Errorf("IsTerminal() for %s = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestPayment_CanTransitionTo(t *testing.T) {
	tests := []struct {
		name   string
		from   PaymentStatus
		to     PaymentStatus
		wantOK bool
	}{
		// Initiated transitions
		{"initiated -> pending_3ds", StatusInitiated, StatusPending3DS, true},
		{"initiated -> failed", StatusInitiated, StatusFailed, true},
		{"initiated -> completed", StatusInitiated, StatusCompleted, false},
		{"initiated -> refunded", StatusInitiated, StatusRefunded, false},

		// Pending3DS transitions
		{"pending_3ds -> confirmed", StatusPending3DS, StatusConfirmed, true},
		{"pending_3ds -> failed", StatusPending3DS, StatusFailed, true},
		{"pending_3ds -> completed", StatusPending3DS, StatusCompleted, false},

		// Confirmed transitions
		{"confirmed -> completed", StatusConfirmed, StatusCompleted, true},
		{"confirmed -> failed", StatusConfirmed, StatusFailed, true},
		{"confirmed -> refunded", StatusConfirmed, StatusRefunded, false},

		// Completed transitions (only refund)
		{"completed -> refunded", StatusCompleted, StatusRefunded, true},
		{"completed -> failed", StatusCompleted, StatusFailed, false},
		{"completed -> completed", StatusCompleted, StatusCompleted, false},

		// Terminal states — no outgoing
		{"failed -> any", StatusFailed, StatusInitiated, false},
		{"failed -> refunded", StatusFailed, StatusRefunded, false},
		{"refunded -> any", StatusRefunded, StatusInitiated, false},
		{"refunded -> failed", StatusRefunded, StatusFailed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Payment{Status: tt.from}
			if got := p.CanTransitionTo(tt.to); got != tt.wantOK {
				t.Errorf("CanTransitionTo(%s) from %s = %v, want %v",
					tt.to, tt.from, got, tt.wantOK)
			}
		})
	}
}

func TestPayment_CanTransitionTo_UnknownStatus(t *testing.T) {
	p := &Payment{Status: "UNKNOWN"}
	if p.CanTransitionTo(StatusCompleted) {
		t.Error("expected false for unknown source status")
	}
}
