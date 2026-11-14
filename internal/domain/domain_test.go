package domain

import "testing"

func TestFinalizationRequiresIndependentDecisionsForEveryStudent(t *testing.T) {
	a, b, c, file := "lecturer-a", "lecturer-b", "lecturer-c", "file"
	p := Project{Status: "SUBMITTED", SignedDescription: true, DescriptionFileID: &file, Students: []Participant{
		{StudentID: "student-a", StudentName: "Mahasiswa A", Slot: 1, SupervisorID: &a, TitleStatus: "APPROVED", ConfirmationStatus: "PENDING"},
		{StudentID: "student-b", StudentName: "Mahasiswa B", Slot: 1, SupervisorID: &b, TitleStatus: "APPROVED", ConfirmationStatus: "APPROVED"},
	}}
	if ReadyToFinalize(p) == nil {
		t.Fatal("title approval must not imply willingness")
	}
	p.Students[0].ConfirmationStatus = "APPROVED"
	if e := ReadyToFinalize(p); e != nil {
		t.Fatal(e)
	}
	p.Students = append(p.Students, Participant{StudentID: "student-a", Slot: 2, SupervisorID: &c, TitleStatus: "APPROVED", ConfirmationStatus: "APPROVED"})
	if e := ReadyToFinalize(p); e != nil {
		t.Fatal(e)
	}
	p.Students[2].SupervisorID = &a
	if ReadyToFinalize(p) == nil {
		t.Fatal("same supervisor cannot fill two positions for a student")
	}
	p.Students = p.Students[:2]
	p.Students[1].SupervisorID = nil
	if ReadyToFinalize(p) == nil {
		t.Fatal("every student needs their own assignment")
	}
}
func TestGroupValidation(t *testing.T) {
	a, b, c := "89175b7a-529a-48d0-a970-1c89e2174c01", "89175b7a-529a-48d0-a970-1c89e2174c02", "89175b7a-529a-48d0-a970-1c89e2174c03"
	p := ProjectInput{Title: "Judul capstone", Summary: "Deskripsi capstone kelompok", InterestID: c, Members: []MemberInput{{StudentID: a, Candidate1: &c}, {StudentID: b}}}
	if e := p.Validate(a); e != nil {
		t.Fatal(e)
	}
	p.Members[1].StudentID = a
	if p.Validate(a) == nil {
		t.Fatal("duplicate student")
	}
	p.Members[1].StudentID = b
	p.Members[0].Candidate2 = &c
	if p.Validate(a) == nil {
		t.Fatal("duplicate candidate for same student")
	}
}
