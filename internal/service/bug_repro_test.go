package service

import (
	"testing"

	"volunteer-scheduler/internal/domain"
)

func TestCancellingWaitingRegistrationDoesNotConfirmAnother(t *testing.T) {
	svc := newTestService(t)
	first := createTestVolunteer(svc, "Alice", []string{"medical"})
	second := createTestVolunteer(svc, "Bob", []string{"medical"})
	third := createTestVolunteer(svc, "Carol", []string{"medical"})
	activity := createTestActivity(svc, "Clinic", []ShiftInput{
		createTestShiftInput("2025-02-02T09:00:00Z", "2025-02-02T12:00:00Z", []string{"medical"}, 1),
	})
	r1, _ := svc.RegisterVolunteer(first.ID, activity.Shifts[0].ID)
	r2, _ := svc.RegisterVolunteer(second.ID, activity.Shifts[0].ID)
	r3, _ := svc.RegisterVolunteer(third.ID, activity.Shifts[0].ID)
	_, _ = svc.ConfirmRegistration(r1.ID)

	if _, err := svc.CancelRegistration(r2.ID); err != nil {
		t.Fatalf("cancel waiting registration: %v", err)
	}
	remaining, _ := svc.store.GetRegistration(r3.ID)
	if remaining.State != domain.StateRegistered {
		t.Fatalf("remaining waiting registration state = %s, want %s", remaining.State, domain.StateRegistered)
	}
}
