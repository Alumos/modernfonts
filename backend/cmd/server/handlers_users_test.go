package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestAdminManagesUsersAndInvalidatesOldSessions(t *testing.T) {
	rt, db := newAuthTestRuntime(t)
	admin := createAuthTestAccount(t, db, "admin-test", AdminRoleAdmin)
	router := authTestRouter(rt)

	created := performAuthTestRequest(t, rt, router, &admin, http.MethodPost, "/api/admin/users", `{"username":"reader","password":"reader-password"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create user status = %d, body = %s", created.Code, created.Body.String())
	}
	var createdResponse struct {
		User Admin `json:"user"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdResponse); err != nil {
		t.Fatalf("decode created user: %v", err)
	}
	user := createdResponse.User
	if user.Role != AdminRoleUser || user.MustChangeCredentials || user.Status != AdminStatusActive {
		t.Fatalf("created user = %+v", user)
	}
	if user.PasswordHash != "" || user.AuthVersion != 0 {
		t.Fatal("user response exposed private authentication fields")
	}
	if duplicate := performAuthTestRequest(t, rt, router, &admin, http.MethodPost, "/api/admin/users", `{"username":"reader","password":"another-password"}`); duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, body = %s", duplicate.Code, duplicate.Body.String())
	}
	if weak := performAuthTestRequest(t, rt, router, &admin, http.MethodPost, "/api/admin/users", `{"username":"weak-user","password":"123"}`); weak.Code != http.StatusBadRequest {
		t.Fatalf("weak password status = %d, body = %s", weak.Code, weak.Body.String())
	}

	if err := db.First(&user, user.ID).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	oldSession := user
	disabled := performAuthTestRequest(t, rt, router, &admin, http.MethodPatch, "/api/admin/users/"+itoa(user.ID)+"/status", `{"status":"disabled"}`)
	if disabled.Code != http.StatusOK {
		t.Fatalf("disable status = %d, body = %s", disabled.Code, disabled.Body.String())
	}
	if me := performAuthTestRequest(t, rt, router, &oldSession, http.MethodGet, "/api/auth/me", ""); me.Code != http.StatusUnauthorized {
		t.Fatalf("disabled old session status = %d, body = %s", me.Code, me.Body.String())
	}
	if login := performAuthTestRequest(t, rt, router, nil, http.MethodPost, "/api/auth/login", `{"username":"reader","password":"reader-password"}`); login.Code != http.StatusUnauthorized {
		t.Fatalf("disabled login status = %d, body = %s", login.Code, login.Body.String())
	}

	enabled := performAuthTestRequest(t, rt, router, &admin, http.MethodPatch, "/api/admin/users/"+itoa(user.ID)+"/status", `{"status":"active"}`)
	if enabled.Code != http.StatusOK {
		t.Fatalf("enable status = %d, body = %s", enabled.Code, enabled.Body.String())
	}
	if err := db.First(&user, user.ID).Error; err != nil {
		t.Fatalf("reload enabled user: %v", err)
	}
	beforeReset := user
	if me := performAuthTestRequest(t, rt, router, &user, http.MethodGet, "/api/auth/me", ""); me.Code != http.StatusOK {
		t.Fatalf("enabled session status = %d, body = %s", me.Code, me.Body.String())
	}

	reset := performAuthTestRequest(t, rt, router, &admin, http.MethodPost, "/api/admin/users/"+itoa(user.ID)+"/reset-password", `{"password":"new-reader-password"}`)
	if reset.Code != http.StatusOK {
		t.Fatalf("reset password status = %d, body = %s", reset.Code, reset.Body.String())
	}
	if me := performAuthTestRequest(t, rt, router, &beforeReset, http.MethodGet, "/api/auth/me", ""); me.Code != http.StatusUnauthorized {
		t.Fatalf("pre-reset session status = %d, body = %s", me.Code, me.Body.String())
	}
	if oldLogin := performAuthTestRequest(t, rt, router, nil, http.MethodPost, "/api/auth/login", `{"username":"reader","password":"reader-password"}`); oldLogin.Code != http.StatusUnauthorized {
		t.Fatalf("old password login status = %d, body = %s", oldLogin.Code, oldLogin.Body.String())
	}
	if newLogin := performAuthTestRequest(t, rt, router, nil, http.MethodPost, "/api/auth/login", `{"username":"reader","password":"new-reader-password"}`); newLogin.Code != http.StatusOK {
		t.Fatalf("new password login status = %d, body = %s", newLogin.Code, newLogin.Body.String())
	}

	adminStatus := performAuthTestRequest(t, rt, router, &admin, http.MethodPatch, "/api/admin/users/"+itoa(admin.ID)+"/status", `{"status":"disabled"}`)
	if adminStatus.Code != http.StatusNotFound {
		t.Fatalf("managed admin status = %d, body = %s", adminStatus.Code, adminStatus.Body.String())
	}
}

func TestFirstLoginAdminCanOnlyAccessCredentialRoutes(t *testing.T) {
	rt, db := newAuthTestRuntime(t)
	admin := createAuthTestAccount(t, db, "first-login", AdminRoleAdmin)
	admin.MustChangeCredentials = true
	if err := db.Model(&Admin{}).Where("id = ?", admin.ID).Update("must_change_credentials", true).Error; err != nil {
		t.Fatalf("mark first login: %v", err)
	}
	router := authTestRouter(rt)
	if me := performAuthTestRequest(t, rt, router, &admin, http.MethodGet, "/api/admin/me", ""); me.Code != http.StatusOK {
		t.Fatalf("admin me status = %d, body = %s", me.Code, me.Body.String())
	}
	if users := performAuthTestRequest(t, rt, router, &admin, http.MethodGet, "/api/admin/users", ""); users.Code != http.StatusForbidden {
		t.Fatalf("first-login users status = %d, body = %s", users.Code, users.Body.String())
	}
}

func itoa(value uint) string {
	return fmt.Sprint(value)
}
