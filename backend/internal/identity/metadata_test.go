package identity

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestClientCannotChoosePlatformOwnershipOrClubAssignment(t *testing.T) {
	service := New(nil, nil, false)
	mux := http.NewServeMux()
	service.Mount(mux)
	for _, field := range []map[string]any{
		{"isPlatformOwner": true},
		{"assignedClubId": "a0000000-0000-4000-8000-000000000001"},
	} {
		body := map[string]any{"email": "synthetic@example.invalid", "displayName": "Synthetic Person", "password": "Synthetic-password-metadata"}
		for key, value := range field {
			body[key] = value
		}
		status(t, apiRequest(t, mux, "POST", "/v1/auth/register", body), 400)
	}
	data, err := json.Marshal(User{ID: "synthetic"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"isPlatformOwner":false`) {
		t.Fatal("regular user must explicitly report no platform ownership")
	}
	if strings.Contains(string(data), `"assignedClubId"`) {
		t.Fatal("unassigned user should omit club identifier")
	}
}

func TestAccountMetadataKeepsPlatformOwnerSeparateFromAssignedClientWithPostgres(t *testing.T) {
	for _, isOwner := range []bool{false, true} {
		name := "assigned client"
		if isOwner {
			name = "platform owner"
		}
		t.Run(name, func(t *testing.T) {
			ctx, service, mail, handler := integrationService(t)
			const password = "Synthetic-password-metadata"
			register := apiRequest(t, handler, "POST", "/v1/auth/register", map[string]string{
				"email": "metadata@example.invalid", "displayName": "Synthetic Metadata", "password": password,
			})
			status(t, register, 201)
			account := responseUser(t, register)
			if account.IsPlatformOwner || account.AssignedClubID != "" {
				t.Fatal("new account acquired platform or club privileges")
			}
			originalCookie := sessionCookie(t, register)
			status(t, apiRequest(t, handler, "POST", "/v1/auth/verify-email", map[string]string{"token": mail.latestToken(t)}), 204)
			const clubID = "a0000000-0000-4000-8000-000000000011"
			if _, err := service.db.Exec(ctx, `INSERT INTO clubs(id,name) VALUES($1,'Synthetic assigned club')`, clubID); err != nil {
				t.Fatal(err)
			}
			if _, err := service.db.Exec(ctx, `UPDATE users SET assigned_club_id=$2 WHERE id=$1`, account.ID, clubID); err != nil {
				t.Fatal(err)
			}
			if isOwner {
				// Simulate legacy ownership promotion after a client assignment. The public
				// identity must remain an operator even if an old client pointer survives.
				if _, err := service.db.Exec(ctx, `INSERT INTO platform_owners(user_id) VALUES($1)`, account.ID); err != nil {
					t.Fatal(err)
				}
			}
			assertMetadata := func(want string, user User) {
				t.Helper()
				if user.ID != account.ID || !user.EmailVerified || user.IsPlatformOwner != isOwner || user.AssignedClubID != want {
					t.Fatalf("server metadata stale or incorrect: user=%s verified=%v owner=%v club=%s", user.ID, user.EmailVerified, user.IsPlatformOwner, user.AssignedClubID)
				}
			}
			want := clubID
			if isOwner {
				want = ""
			}
			me := apiRequest(t, handler, "GET", "/v1/me", nil, originalCookie)
			status(t, me, 200)
			assertMetadata(want, responseUser(t, me))
			if isOwner && strings.Contains(me.Body.String(), `"assignedClubId"`) {
				t.Fatal("platform operator response exposes a client assignment")
			}
			login := apiRequest(t, handler, "POST", "/v1/auth/login", map[string]string{"email": account.Email, "password": password})
			status(t, login, 200)
			assertMetadata(want, responseUser(t, login))
			refresh := apiRequest(t, handler, "POST", "/v1/auth/refresh", nil, sessionCookie(t, login))
			status(t, refresh, 200)
			assertMetadata(want, responseUser(t, refresh))
			// Clear only this synthetic assignment: session reads must not cache scope.
			if _, err := service.db.Exec(ctx, `UPDATE users SET assigned_club_id=NULL WHERE id=$1`, account.ID); err != nil {
				t.Fatal(err)
			}
			refreshed := apiRequest(t, handler, "POST", "/v1/auth/refresh", nil, sessionCookie(t, refresh))
			status(t, refreshed, 200)
			assertMetadata("", responseUser(t, refreshed))
			current := apiRequest(t, handler, "GET", "/v1/me", nil, originalCookie)
			status(t, current, 200)
			assertMetadata("", responseUser(t, current))
		})
	}
}
