package app_test

import (
	"testing"
	"time"

	"github.com/jonathanselby/garage-crm/internal/app"
)

func coolingIDs(t *testing.T, s *app.Service, brand string) []string {
	t.Helper()
	rows, err := s.CoolingItems(brand)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, row := range rows {
		ids = append(ids, app.String(row["entity"])+":"+app.String(row["id"]))
	}
	return ids
}

func expectCooling(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("cooling: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cooling: got %v want %v", got, want)
		}
	}
}

func TestCoolingTasksActivityClosedAndContactFallback(t *testing.T) {
	s, _ := setup(t)
	brand := create(t, s, "brands", map[string]string{"name": "StagZero"})
	person := create(t, s, "people", map[string]string{"name": "CISO"})
	if err := s.LinkContactToBrand(brand, person, "Prospect", "unknown", false); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-16 * 24 * time.Hour).Format(time.RFC3339Nano)
	if err := s.DB.Exec("UPDATE people SET created_at=? WHERE id=?", old, person); err != nil {
		t.Fatal(err)
	}
	opp := create(t, s, "opportunities", map[string]string{"title": "Pilot", "brand_id": brand, "person_id": person})
	if err := s.DB.Exec("UPDATE opportunities SET updated_at=? WHERE id=?", old, opp); err != nil {
		t.Fatal(err)
	}
	expectCooling(t, coolingIDs(t, s, brand), "opportunities:"+opp)
	task := create(t, s, "tasks", map[string]string{"title": "Next step", "brand_id": brand, "opportunity_id": opp})
	expectCooling(t, coolingIDs(t, s, brand))
	if err := s.DB.Exec("UPDATE tasks SET status='Done' WHERE id=?", task); err != nil {
		t.Fatal(err)
	}
	expectCooling(t, coolingIDs(t, s, brand), "opportunities:"+opp)
	if err := s.AddActivity(brand, person, "", "", opp, "", "Call", "Discussion", "", ""); err != nil {
		t.Fatal(err)
	}
	expectCooling(t, coolingIDs(t, s, brand))
	if err := s.DB.Exec("UPDATE activities SET occurred_at=? WHERE opportunity_id=?", old, opp); err != nil {
		t.Fatal(err)
	}
	expectCooling(t, coolingIDs(t, s, brand), "opportunities:"+opp)
	if err := s.DB.Exec("UPDATE opportunities SET stage='Won' WHERE id=?", opp); err != nil {
		t.Fatal(err)
	}
	expectCooling(t, coolingIDs(t, s, brand), "people:"+person)
	if err := s.DB.Exec("UPDATE people SET archived=1 WHERE id=?", person); err != nil {
		t.Fatal(err)
	}
	expectCooling(t, coolingIDs(t, s, brand))
}

func TestCoolingBrandIsolation(t *testing.T) {
	s, _ := setup(t)
	a := create(t, s, "brands", map[string]string{"name": "A"})
	b := create(t, s, "brands", map[string]string{"name": "B"})
	person := create(t, s, "people", map[string]string{"name": "Contact"})
	for _, brand := range []string{a, b} {
		if err := s.LinkContactToBrand(brand, person, "Prospect", "unknown", false); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().UTC().Add(-16 * 24 * time.Hour).Format(time.RFC3339Nano)
	if err := s.DB.Exec("UPDATE people SET created_at=? WHERE id=?", old, person); err != nil {
		t.Fatal(err)
	}
	if err := s.AddActivity(b, person, "", "", "", "", "Note", "B interaction", "", ""); err != nil {
		t.Fatal(err)
	}
	expectCooling(t, coolingIDs(t, s, a), "people:"+person)
	expectCooling(t, coolingIDs(t, s, b))
	task := create(t, s, "tasks", map[string]string{"title": "Next action", "brand_id": a, "person_id": person})
	expectCooling(t, coolingIDs(t, s, a))
	if err := s.DB.Exec("UPDATE tasks SET archived=1 WHERE id=?", task); err != nil {
		t.Fatal(err)
	}
	expectCooling(t, coolingIDs(t, s, a), "people:"+person)
	if err := s.LinkContactToBrand(a, person, "Client", "unknown", false); err != nil {
		t.Fatal(err)
	}
	expectCooling(t, coolingIDs(t, s, a))
}

func TestCoolingRecentAndLost(t *testing.T) {
	s, _ := setup(t)
	brand := create(t, s, "brands", map[string]string{"name": "A"})
	recent := create(t, s, "opportunities", map[string]string{"title": "New", "brand_id": brand})
	lost := create(t, s, "opportunities", map[string]string{"title": "Lost", "brand_id": brand})
	old := time.Now().UTC().Add(-16 * 24 * time.Hour).Format(time.RFC3339Nano)
	if err := s.DB.Exec("UPDATE opportunities SET updated_at=?,stage='Lost' WHERE id=?", old, lost); err != nil {
		t.Fatal(err)
	}
	expectCooling(t, coolingIDs(t, s, brand))
	if err := s.DB.Exec("UPDATE opportunities SET updated_at=? WHERE id=?", old, recent); err != nil {
		t.Fatal(err)
	}
	expectCooling(t, coolingIDs(t, s, brand), "opportunities:"+recent)
}
