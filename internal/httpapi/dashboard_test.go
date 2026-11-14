package httpapi

import (
	"capstone/capstone-service/internal/domain"
	"testing"
)

func TestDashboardRolePrecedence(t *testing.T) {
	for _, test := range []struct {
		roles []string
		scope string
	}{
		{[]string{"MAHASISWA"}, "student"}, {[]string{"DOSEN"}, "lecturer"}, {[]string{"DOSEN", "KOPRODI"}, "koprodi"},
		{[]string{"DOSEN", "KOPRODI", "KOORDINATOR_TA"}, "coordinator"}, {[]string{"TU"}, "tu"}, {[]string{"ADMIN"}, "admin"},
	} {
		if got := dashboardScope(domain.User{Roles: test.roles}); got != test.scope {
			t.Fatalf("roles=%v got=%s want=%s", test.roles, got, test.scope)
		}
	}
}
