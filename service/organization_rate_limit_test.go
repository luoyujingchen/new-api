package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetEffectiveRateLimitPrefersChildDepartmentOverParent(t *testing.T) {
	setupOrganizationRateLimitTestDB(t)
	svc := GetOrganizationRateLimitService()

	company := createTestCompany(t, "child-priority")
	parentDept := createTestDepartment(t, company.Id, "Parent", nil)
	childDept := createTestDepartment(t, company.Id, "Child", &parentDept.Id)
	user := createTestOrgUser(t, company.Id, childDept.Id, "child-priority")

	require.NoError(t, svc.Create(&model.OrganizationRateLimit{
		OrgType: "department",
		OrgId:   parentDept.Id,
		TimeSlots: model.TimeSlots{{
			StartTime: "00:00",
			EndTime:   "23:59",
		}},
		Rpms:   model.Rpms{30},
		Status: 1,
	}))
	require.NoError(t, svc.Create(&model.OrganizationRateLimit{
		OrgType: "department",
		OrgId:   childDept.Id,
		TimeSlots: model.TimeSlots{{
			StartTime: "00:00",
			EndTime:   "23:59",
		}},
		Rpms:   model.Rpms{20},
		Status: 1,
	}))

	effective, err := svc.GetEffectiveRateLimit(user.Id, "", time.Date(2026, time.May, 21, 10, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.NotNil(t, effective)
	require.Equal(t, int64(childDept.Id), effective.OrgId)
	require.Equal(t, 20, effective.Rpm)
	require.Equal(t, "department", effective.Source)
}

func TestGetEffectiveRateLimitPrefersExactModelRuleOverWildcard(t *testing.T) {
	setupOrganizationRateLimitTestDB(t)
	svc := GetOrganizationRateLimitService()

	company := createTestCompany(t, "model-priority")
	dept := createTestDepartment(t, company.Id, "Engineering", nil)
	user := createTestOrgUser(t, company.Id, dept.Id, "model-priority")
	modelName := "gpt-4o-external"

	require.NoError(t, svc.Create(&model.OrganizationRateLimit{
		OrgType: "department",
		OrgId:   dept.Id,
		TimeSlots: model.TimeSlots{{
			StartTime: "00:00",
			EndTime:   "23:59",
		}},
		Rpms:   model.Rpms{20},
		Status: 1,
	}))
	require.NoError(t, svc.Create(&model.OrganizationRateLimit{
		OrgType:   "department",
		OrgId:     dept.Id,
		ModelName: &modelName,
		TimeSlots: model.TimeSlots{{
			StartTime: "00:00",
			EndTime:   "23:59",
		}},
		Rpms:   model.Rpms{5},
		Status: 1,
	}))

	effective, err := svc.GetEffectiveRateLimit(user.Id, modelName, time.Date(2026, time.May, 21, 10, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.NotNil(t, effective)
	require.Equal(t, modelName, effective.ModelName)
	require.Equal(t, 5, effective.Rpm)
}

func setupOrganizationRateLimitTestDB(t *testing.T) {
	t.Helper()
	require.NoError(t, model.DB.AutoMigrate(
		&model.Company{},
		&model.Department{},
		&model.OrganizationRateLimit{},
		&model.Model{},
		&model.User{},
	))
	InvalidateAllOrgRateLimitCache()
	t.Cleanup(func() {
		session := model.DB.Session(&gorm.Session{AllowGlobalUpdate: true})
		require.NoError(t, session.Unscoped().Delete(&model.OrganizationRateLimit{}).Error)
		require.NoError(t, session.Unscoped().Delete(&model.User{}).Error)
		require.NoError(t, session.Unscoped().Delete(&model.Department{}).Error)
		require.NoError(t, session.Unscoped().Delete(&model.Company{}).Error)
		require.NoError(t, session.Unscoped().Delete(&model.Model{}).Error)
		InvalidateAllOrgRateLimitCache()
	})
}

func createTestCompany(t *testing.T, suffix string) *model.Company {
	t.Helper()
	company := &model.Company{
		Name:   fmt.Sprintf("Company-%s", suffix),
		Code:   fmt.Sprintf("code-%s", suffix),
		Status: 1,
	}
	require.NoError(t, company.Insert())
	return company
}

func createTestDepartment(t *testing.T, companyID int64, name string, parentID *int64) *model.Department {
	t.Helper()
	dept := &model.Department{
		CompanyId: companyID,
		Name:      name,
		ParentId:  parentID,
		Status:    1,
	}
	require.NoError(t, dept.Insert())
	return dept
}

func createTestOrgUser(t *testing.T, companyID, departmentID int64, suffix string) *model.User {
	t.Helper()
	companyRef := companyID
	departmentRef := departmentID
	user := &model.User{
		Username:     fmt.Sprintf("user_%s", suffix),
		Password:     "password123",
		DisplayName:  fmt.Sprintf("User %s", suffix),
		Status:       1,
		Role:         1,
		Group:        "default",
		CompanyId:    &companyRef,
		DepartmentId: &departmentRef,
	}
	require.NoError(t, model.DB.Create(user).Error)
	return user
}

func createTestModelMeta(t *testing.T, modelName, suffix string) *model.Model {
	t.Helper()
	modelMeta := &model.Model{
		ModelName:    modelName,
		Description:  fmt.Sprintf("model-%s", suffix),
		Status:       1,
		SyncOfficial: 1,
	}
	require.NoError(t, modelMeta.Insert())
	return modelMeta
}
