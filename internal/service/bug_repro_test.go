package service

import "testing"

func TestConfirmedRegistrationStillBlocksOverlappingShift(t *testing.T) {
	svc := newTestService(t)
	volunteer := createTestVolunteer(svc, "Alice", []string{"general"})
	activity := createTestActivity(svc, "Overlap", []ShiftInput{
		createTestShiftInput("2025-02-05T09:00:00Z", "2025-02-05T11:00:00Z", []string{"general"}, 1),
		createTestShiftInput("2025-02-05T10:00:00Z", "2025-02-05T12:00:00Z", []string{"general"}, 1),
	})
	first, _ := svc.RegisterVolunteer(volunteer.ID, activity.Shifts[0].ID)
	if _, err := svc.ConfirmRegistration(first.ID); err != nil {
		t.Fatalf("confirm first registration: %v", err)
	}

	if _, err := svc.RegisterVolunteer(volunteer.ID, activity.Shifts[1].ID); err == nil {
		t.Fatal("overlapping shift accepted while first registration is confirmed")
	}
	_, _ = svc.CheckInRegistration(first.ID)
	_, _ = svc.SettleRegistration(first.ID)
	if _, err := svc.RegisterVolunteer(volunteer.ID, activity.Shifts[1].ID); err != nil {
		t.Fatalf("overlapping shift should be available after settlement: %v", err)
	}
}
