package service

import (
	"testing"
	"time"

	"volunteer-scheduler/internal/domain"
	"volunteer-scheduler/internal/store"
)

func newTestService(t *testing.T) *Service {
	st := store.NewStore()
	svc := NewService(st)
	return svc
}

func createTestVolunteer(svc *Service, name string, skills []string) *domain.Volunteer {
	v, err := svc.CreateVolunteer(name, skills)
	if err != nil {
		panic(err)
	}
	return v
}

func createTestActivity(svc *Service, name string, shifts []ShiftInput) *domain.Activity {
	a, err := svc.CreateActivity(name, shifts)
	if err != nil {
		panic(err)
	}
	return a
}

func createTestShiftInput(start, end string, skills []string, count int) ShiftInput {
	startTime, _ := time.Parse(time.RFC3339, start)
	endTime, _ := time.Parse(time.RFC3339, end)
	return ShiftInput{
		StartTime:      startTime,
		EndTime:        endTime,
		RequiredSkills: skills,
		RequiredCount:  count,
	}
}

// TestNormalFlow 正常业务链：报名->确认->签到->结算
func TestNormalFlow(t *testing.T) {
	svc := newTestService(t)
	v := createTestVolunteer(svc, "Alice", []string{"medical"})
	a := createTestActivity(svc, "Health Camp", []ShiftInput{
		createTestShiftInput("2025-01-01T09:00:00Z", "2025-01-01T12:00:00Z", []string{"medical"}, 1),
	})
	shift := a.Shifts[0]

	// 报名
	reg, err := svc.RegisterVolunteer(v.ID, shift.ID)
	if err != nil {
		t.Fatalf("unexpected error when registering: %v", err)
	}
	if reg.State != domain.StateRegistered {
		t.Fatalf("expected state %s, got %s", domain.StateRegistered, reg.State)
	}

	// 确认
	reg, err = svc.ConfirmRegistration(reg.ID)
	if err != nil {
		t.Fatalf("failed to confirm: %v", err)
	}
	if reg.State != domain.StateConfirmed {
		t.Fatalf("expected state %s, got %s", domain.StateConfirmed, reg.State)
	}

	// 签到
	reg, err = svc.CheckInRegistration(reg.ID)
	if err != nil {
		t.Fatalf("failed to check in: %v", err)
	}
	if reg.State != domain.StateCheckedIn {
		t.Fatalf("expected state %s, got %s", domain.StateCheckedIn, reg.State)
	}

	// 结算
	reg, err = svc.SettleRegistration(reg.ID)
	if err != nil {
		t.Fatalf("failed to settle: %v", err)
	}
	if reg.State != domain.StateSettled {
		t.Fatalf("expected state %s, got %s", domain.StateSettled, reg.State)
	}
}

// TestFailedTransitionKeepsState 失败后状态不变
func TestFailedTransitionKeepsState(t *testing.T) {
	svc := newTestService(t)
	v := createTestVolunteer(svc, "Bob", []string{"tech"})
	a := createTestActivity(svc, "Tech Aid", []ShiftInput{
		createTestShiftInput("2025-01-02T09:00:00Z", "2025-01-02T11:00:00Z", []string{"tech"}, 1),
	})
	shift := a.Shifts[0]
	reg, err := svc.RegisterVolunteer(v.ID, shift.ID)
	if err != nil {
		t.Fatal(err)
	}
	// 尝试从 registered 直接签到，应该失败
	_, err = svc.CheckInRegistration(reg.ID)
	if err == nil {
		t.Fatal("expected error when skipping confirm")
	}
	// 检查状态未变
	regAfter, _ := svc.store.GetRegistration(reg.ID)
	if regAfter.State != domain.StateRegistered {
		t.Fatalf("expected state remain %s, got %s", domain.StateRegistered, regAfter.State)
	}

	// 尝试从 confirmed 跳到 settled
	_, err = svc.ConfirmRegistration(reg.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.SettleRegistration(reg.ID)
	if err == nil {
		t.Fatal("expected error when skipping checkin")
	}
	regAfter2, _ := svc.store.GetRegistration(reg.ID)
	if regAfter2.State != domain.StateConfirmed {
		t.Fatalf("expected state remain %s, got %s", domain.StateConfirmed, regAfter2.State)
	}
}

// TestSkillMismatch 技能不匹配不能报名
func TestSkillMismatch(t *testing.T) {
	svc := newTestService(t)
	v := createTestVolunteer(svc, "Carol", []string{"language"})
	a := createTestActivity(svc, "Medical Camp", []ShiftInput{
		createTestShiftInput("2025-01-03T09:00:00Z", "2025-01-03T12:00:00Z", []string{"medical"}, 1),
	})
	shift := a.Shifts[0]
	_, err := svc.RegisterVolunteer(v.ID, shift.ID)
	if err == nil {
		t.Fatal("expected error for skill mismatch")
	}
}

// TestTimeConflict 时间冲突不能报名
func TestTimeConflict(t *testing.T) {
	svc := newTestService(t)
	v := createTestVolunteer(svc, "Dave", []string{"general"})
	a := createTestActivity(svc, "Concurrent Shifts", []ShiftInput{
		createTestShiftInput("2025-01-04T09:00:00Z", "2025-01-04T11:00:00Z", []string{"general"}, 1),
		createTestShiftInput("2025-01-04T10:00:00Z", "2025-01-04T12:00:00Z", []string{"general"}, 1),
	})
	// 报名第一个班次
	reg1, err := svc.RegisterVolunteer(v.ID, a.Shifts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	// 尝试报名重叠的第二个班次
	_, err = svc.RegisterVolunteer(v.ID, a.Shifts[1].ID)
	if err == nil {
		t.Fatal("expected time conflict error")
	}
	// 确认第一报名，不影响
	_, err = svc.ConfirmRegistration(reg1.ID)
	if err != nil {
		t.Fatal(err)
	}
}

// TestCancelAndAutoFill 取消后按报名时间递补
func TestCancelAndAutoFill(t *testing.T) {
	svc := newTestService(t)
	// 两个志愿者都有匹配技能
	v1 := createTestVolunteer(svc, "Eve", []string{"medical"})
	v2 := createTestVolunteer(svc, "Frank", []string{"medical"})
	a := createTestActivity(svc, "Clinic", []ShiftInput{
		createTestShiftInput("2025-01-05T09:00:00Z", "2025-01-05T12:00:00Z", []string{"medical"}, 1),
	})
	shift := a.Shifts[0]
	// v1 先报名，v2 后报名
	r1, _ := svc.RegisterVolunteer(v1.ID, shift.ID)
	r2, _ := svc.RegisterVolunteer(v2.ID, shift.ID)
	// 确认 v1
	_, err := svc.ConfirmRegistration(r1.ID)
	if err != nil {
		t.Fatal(err)
	}
	// 取消 v1，应自动递补 v2 为 confirmed
	_, err = svc.CancelRegistration(r1.ID)
	if err != nil {
		t.Fatal(err)
	}
	// 获取 v2 的最新状态
	updatedR2, _ := svc.store.GetRegistration(r2.ID)
	if updatedR2.State != domain.StateConfirmed {
		t.Fatalf("expected v2 to be confirmed, got %s", updatedR2.State)
	}
	// 且 v2 的确认时间应为取消后的时间
	if updatedR2.ConfirmedAt == nil {
		t.Fatal("expected ConfirmedAt to be set")
	}
}

// TestActivityStatistics 统计覆盖率与缺口
func TestActivityStatistics(t *testing.T) {
	svc := newTestService(t)
	v1 := createTestVolunteer(svc, "Grace", []string{"tech"})
	v2 := createTestVolunteer(svc, "Henry", []string{"tech"})
	v3 := createTestVolunteer(svc, "Ivy", []string{"tech"})
	a := createTestActivity(svc, "Tech Support", []ShiftInput{
		createTestShiftInput("2025-01-06T09:00:00Z", "2025-01-06T10:00:00Z", []string{"tech"}, 2),
		createTestShiftInput("2025-01-06T11:00:00Z", "2025-01-06T12:00:00Z", []string{"tech"}, 1),
	})
	shift1 := a.Shifts[0]
	shift2 := a.Shifts[1]

	// 报名并确认 3 个志愿者到 shift1 (需要2人)，1人到 shift2
	r1, _ := svc.RegisterVolunteer(v1.ID, shift1.ID)
	r2, _ := svc.RegisterVolunteer(v2.ID, shift1.ID)
	r3, _ := svc.RegisterVolunteer(v3.ID, shift2.ID)

	// 确认两个
	svc.ConfirmRegistration(r1.ID)
	svc.ConfirmRegistration(r2.ID)
	// r3 也确认
	svc.ConfirmRegistration(r3.ID)

	// 签到 r1
	svc.CheckInRegistration(r1.ID)
	// 结算 r1
	svc.SettleRegistration(r1.ID)

	// 统计
	stats, err := svc.GetActivityStatistics(a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalRequired != 3 {
		t.Errorf("expected total required 3, got %d", stats.TotalRequired)
	}
	// 有效人数: settled(1) + confirmed(2) = 3? r1 是 settled，r2 是 confirmed，r3 是 confirmed，总共3人
	// coverage = 3/3 = 1.0
	if stats.CoverageRate != 1.0 {
		t.Errorf("expected coverage 1.0, got %f", stats.CoverageRate)
	}
	if stats.Shortage != 0 {
		t.Errorf("expected shortage 0, got %d", stats.Shortage)
	}

	// 如果只确认2人，则覆盖2/3
	// 但我们已经确认了3人，所以没问题
	// 测试一个不完整的场景：再创建一个活动，只确认1人
	a2 := createTestActivity(svc, "Incomplete", []ShiftInput{
		createTestShiftInput("2025-01-07T09:00:00Z", "2025-01-07T12:00:00Z", []string{"tech"}, 5),
	})
	v4 := createTestVolunteer(svc, "Jack", []string{"tech"})
	reg4, _ := svc.RegisterVolunteer(v4.ID, a2.Shifts[0].ID)
	svc.ConfirmRegistration(reg4.ID)
	stats2, err := svc.GetActivityStatistics(a2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stats2.TotalRequired != 5 {
		t.Errorf("expected total required 5, got %d", stats2.TotalRequired)
	}
	if stats2.Shortage != 4 {
		t.Errorf("expected shortage 4, got %d", stats2.Shortage)
	}
	if stats2.CoverageRate != 0.2 {
		t.Errorf("expected coverage 0.2, got %f", stats2.CoverageRate)
	}
}
