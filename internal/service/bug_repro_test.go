package service

import "testing"

func TestActivityStatisticsExcludeOtherActivityShifts(t *testing.T) {
	svc := newTestService(t)
	volunteer := createTestVolunteer(svc, "Alice", []string{"tech"})
	target := createTestActivity(svc, "Target", []ShiftInput{
		createTestShiftInput("2025-02-04T09:00:00Z", "2025-02-04T10:00:00Z", []string{"tech"}, 1),
	})
	_ = createTestActivity(svc, "Other", []ShiftInput{
		createTestShiftInput("2025-02-04T11:00:00Z", "2025-02-04T12:00:00Z", []string{"tech"}, 4),
	})
	registration, _ := svc.RegisterVolunteer(volunteer.ID, target.Shifts[0].ID)
	_, _ = svc.ConfirmRegistration(registration.ID)

	stats, err := svc.GetActivityStatistics(target.ID)
	if err != nil {
		t.Fatalf("get target statistics: %v", err)
	}
	if stats.TotalRequired != 1 || stats.CoverageRate != 1 {
		t.Fatalf("target statistics total=%d coverage=%v, want 1 and 1", stats.TotalRequired, stats.CoverageRate)
	}
}
