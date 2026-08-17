package service

import "testing"

func TestRejectedConflictRegistrationDoesNotBlockRetry(t *testing.T) {
	svc := newTestService(t)
	volunteer := createTestVolunteer(svc, "Alice", []string{"general"})
	activity := createTestActivity(svc, "Overlap", []ShiftInput{
		createTestShiftInput("2025-02-01T09:00:00Z", "2025-02-01T11:00:00Z", []string{"general"}, 1),
		createTestShiftInput("2025-02-01T10:00:00Z", "2025-02-01T12:00:00Z", []string{"general"}, 1),
	})
	first, err := svc.RegisterVolunteer(volunteer.ID, activity.Shifts[0].ID)
	if err != nil {
		t.Fatalf("register first shift: %v", err)
	}
	if _, err := svc.RegisterVolunteer(volunteer.ID, activity.Shifts[1].ID); err == nil {
		t.Fatal("overlapping registration unexpectedly succeeded")
	}
	_, _ = svc.ConfirmRegistration(first.ID)
	_, _ = svc.CheckInRegistration(first.ID)
	_, _ = svc.SettleRegistration(first.ID)

	if _, err := svc.RegisterVolunteer(volunteer.ID, activity.Shifts[1].ID); err != nil {
		t.Fatalf("retry after original shift settled: %v", err)
	}
}
