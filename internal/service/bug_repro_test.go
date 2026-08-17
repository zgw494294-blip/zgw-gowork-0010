package service

import (
	"testing"
	"time"

	"volunteer-scheduler/internal/domain"
)

func TestAutoFillConfirmsEarliestWaitingVolunteer(t *testing.T) {
	svc := newTestService(t)
	first := createTestVolunteer(svc, "Alice", []string{"medical"})
	second := createTestVolunteer(svc, "Bob", []string{"medical"})
	third := createTestVolunteer(svc, "Carol", []string{"medical"})
	activity := createTestActivity(svc, "Clinic", []ShiftInput{
		createTestShiftInput("2025-02-03T09:00:00Z", "2025-02-03T12:00:00Z", []string{"medical"}, 1),
	})
	r1, _ := svc.RegisterVolunteer(first.ID, activity.Shifts[0].ID)
	r2, _ := svc.RegisterVolunteer(second.ID, activity.Shifts[0].ID)
	r3, _ := svc.RegisterVolunteer(third.ID, activity.Shifts[0].ID)
	r2.RegisteredAt = time.Unix(100, 0)
	r3.RegisteredAt = time.Unix(200, 0)
	svc.store.SaveRegistration(r2)
	svc.store.SaveRegistration(r3)
	_, _ = svc.ConfirmRegistration(r1.ID)
	_, _ = svc.CancelRegistration(r1.ID)

	earlier, _ := svc.store.GetRegistration(r2.ID)
	later, _ := svc.store.GetRegistration(r3.ID)
	if earlier.State != domain.StateConfirmed || later.State != domain.StateRegistered {
		t.Fatalf("auto-fill states earlier=%s later=%s, want confirmed/registered", earlier.State, later.State)
	}
}
