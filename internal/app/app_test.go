package app_test

import (
	"github.com/jonathanselby/garage-crm/internal/app"
	"github.com/jonathanselby/garage-crm/internal/portable"
	"github.com/jonathanselby/garage-crm/internal/sqlite"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setup(t *testing.T) (*app.Service, string) {
	t.Helper()
	dbp := filepath.Join(t.TempDir(), "crm.db")
	db, err := sqlite.Open(dbp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = app.Migrate(db); err != nil {
		t.Fatal(err)
	}
	return &app.Service{DB: db}, dbp
}
func create(t *testing.T, s *app.Service, typ string, v map[string]string) string {
	t.Helper()
	id, err := s.Save(typ, "", v, 0)
	if err != nil {
		t.Fatalf("create %s: %v", typ, err)
	}
	return id
}
func TestMultiBrandAndCampaignConsent(t *testing.T) {
	s, _ := setup(t)
	a := create(t, s, "brands", map[string]string{"name": "StagZero", "currency": "AUD"})
	b := create(t, s, "brands", map[string]string{"name": "Contracting", "currency": "GBP"})
	p := create(t, s, "people", map[string]string{"name": "Alice <script>", "email": "alice@example.com"})
	if err := s.LinkContactToBrand(a, p, "Prospect", "yes", false); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkContactToBrand(b, p, "Client", "unknown", false); err != nil {
		t.Fatal(err)
	}
	c := create(t, s, "campaigns", map[string]string{"name": "Launch", "brand_id": a, "currency": "AUD"})
	d := create(t, s, "campaigns", map[string]string{"name": "Contracting", "brand_id": b, "currency": "GBP"})
	if n, err := s.ActivateCampaign(c, 1); err != nil || n != 1 {
		t.Fatalf("eligible StagZero %d %v", n, err)
	}
	if n, err := s.ActivateCampaign(d, 1); err != nil || n != 0 {
		t.Fatalf("unknown consent should exclude %d %v", n, err)
	}
	csv, err := s.RecipientCSV(c)
	if err != nil || !strings.Contains(csv, "alice@example.com") {
		t.Fatalf("missing recipient %s: %v", csv, err)
	}
	if err = s.LinkContactToBrand(a, p, "Prospect", "no", true); err != nil {
		t.Fatal(err)
	}
	csv, err = s.RecipientCSV(c)
	if err != nil || strings.Contains(csv, "alice@example.com") {
		t.Fatalf("optout not respected at export: %s %v", csv, err)
	}
	member, err := s.CampaignMembers(c)
	if err != nil || len(member) != 1 {
		t.Fatal("frozen campaign history lost")
	}
}
func TestLeadConversionAndOptimisticConflict(t *testing.T) {
	s, _ := setup(t)
	brand := create(t, s, "brands", map[string]string{"name": "Applications"})
	person := create(t, s, "people", map[string]string{"name": "Bob"})
	id := create(t, s, "leads", map[string]string{"title": "Interested in app", "brand_id": brand, "person_id": person, "status": "New", "currency": "AUD"})
	opp, err := s.ConvertLead(id, 1, true)
	if err != nil || opp == "" {
		t.Fatalf("conversion %s %v", opp, err)
	}
	r, err := s.One("leads", id)
	if err != nil || r["status"] != "Converted" {
		t.Fatal("conversion did not commit")
	}
	same, err := s.ConvertLead(id, 1, true)
	if err != nil || same != opp {
		t.Fatalf("idempotence %s %v", same, err)
	}
	if err = s.SetStatus("opportunities", opp, "Proposal", 1); err != nil {
		t.Fatal(err)
	}
	if err = s.SetStatus("opportunities", opp, "Negotiation", 1); err == nil {
		t.Fatal("stale stage update was accepted")
	}
	events, err := s.DB.Query("SELECT * FROM stage_events WHERE opportunity_id=?", opp)
	if err != nil || len(events) != 2 {
		t.Fatalf("stage history %d %v", len(events), err)
	}
}
func TestPortableRoundTripAllTablesAndBLOBs(t *testing.T) {
	s, _ := setup(t)
	brand := create(t, s, "brands", map[string]string{"name": "App"})
	person := create(t, s, "people", map[string]string{"name": "Alice", "email": "private@example.com"})
	c := create(t, s, "campaigns", map[string]string{"name": "Local", "brand_id": brand})
	if err := s.LinkContactToBrand(brand, person, "Prospect", "yes", false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ActivateCampaign(c, 1); err != nil {
		t.Fatal(err)
	}
	bytes := []byte{0, 1, 2, 3, 10, 255}
	if err := s.DB.Exec(`INSERT INTO attachments(id,entity_type,entity_id,name,media_type,size,sha256,data,created_at) VALUES(?,?,?,?,?,?,?,?,?)`, app.NewID(), "people", person, "test.dat", "application/octet-stream", int64(len(bytes)), sqlite.Checksum(bytes), bytes, app.Now()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "all.zip")
	if err := portable.Export(s.DB, path); err != nil {
		t.Fatal(err)
	}
	if _, err := portable.Inspect(path); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "new.db")
	if err := portable.ImportIntoEmpty(path, dst); err != nil {
		t.Fatalf("import: %v", err)
	}
	other, err := sqlite.Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	for _, table := range app.ExportTables {
		a, _ := s.DB.Query("SELECT * FROM " + table + " ORDER BY rowid")
		b, _ := other.Query("SELECT * FROM " + table + " ORDER BY rowid")
		if len(a) != len(b) {
			t.Fatalf("%s count differs %d %d", table, len(a), len(b))
		}
	}
	files, err := other.Query("SELECT data FROM attachments")
	if err != nil || len(files) != 1 || sqlite.Checksum(files[0]["data"].([]byte)) != sqlite.Checksum(bytes) {
		t.Fatal("attachment bytes lost")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = data[:len(data)-8]
	bad := filepath.Join(t.TempDir(), "bad.zip")
	os.WriteFile(bad, data, 0600)
	if err := portable.ImportIntoEmpty(bad, filepath.Join(t.TempDir(), "new.db")); err == nil {
		t.Fatal("corrupt archive accepted")
	}
}

func TestMergeIsMostRestrictiveAndPreservesCampaignHistory(t *testing.T) {
	s, _ := setup(t)
	brand := create(t, s, "brands", map[string]string{"name": "StagZero"})
	canonical := create(t, s, "people", map[string]string{"name": "Mary Active", "email": "mary@example.com"})
	duplicate := create(t, s, "people", map[string]string{"name": "Mary Older", "email": "older@example.com", "global_dnc": "on"})
	if err := s.LinkContactToBrand(brand, canonical, "Prospect", "yes", false); err != nil {
		t.Fatal(err)
	}
	if err := s.LinkContactToBrand(brand, duplicate, "Prospect", "no", true); err != nil {
		t.Fatal(err)
	}
	campaign := create(t, s, "campaigns", map[string]string{"name": "Original launch", "brand_id": brand})
	if n, err := s.ActivateCampaign(campaign, 1); err != nil || n != 1 {
		t.Fatalf("campaign activation %d %v", n, err)
	}
	if err := s.AddContactDetail(duplicate, "email", "other@example.com", "work"); err != nil {
		t.Fatal(err)
	}
	if err := s.MergePeople(canonical, duplicate); err != nil {
		t.Fatal(err)
	}
	person, _ := s.One("people", canonical)
	if person["global_dnc"] != int64(1) {
		t.Fatalf("global do not contact lost: %v", person["global_dnc"])
	}
	brands, _ := s.ContactBrands(canonical)
	if len(brands) != 1 || brands[0]["consent"] != "no" || brands[0]["optout"] != int64(1) {
		t.Fatalf("permission relaxed: %+v", brands)
	}
	emails, _, _, err := s.ContactDetails(canonical)
	if err != nil || len(emails) != 1 {
		t.Fatalf("secondary email lost: %+v %v", emails, err)
	}
	members, err := s.CampaignMembers(campaign)
	if err != nil || len(members) != 1 || members[0]["person_id"] != canonical {
		t.Fatalf("historical membership changed: %+v %v", members, err)
	}
	csv, err := s.RecipientCSV(campaign)
	if err != nil || strings.Contains(csv, "mary@example.com") {
		t.Fatalf("merged optout ignored: %s / %v", csv, err)
	}
}
func TestPermanentRedactionPortableExport(t *testing.T) {
	s, _ := setup(t)
	brand := create(t, s, "brands", map[string]string{"name": "Contracting"})
	name := "Sensitive Person 9481"
	email := "secret-9481@example.com"
	person := create(t, s, "people", map[string]string{"name": name, "email": email, "notes": "private note 9481"})
	if err := s.LinkContactToBrand(brand, person, "Client", "yes", false); err != nil {
		t.Fatal(err)
	}
	campaign := create(t, s, "campaigns", map[string]string{"name": "Email", "brand_id": brand})
	if _, err := s.ActivateCampaign(campaign, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.AddActivity(brand, person, "", "", "", "", "Meeting", "private note 9481", "Call 9481", "2026-12-20"); err != nil {
		t.Fatal(err)
	}
	if err := s.DB.Exec("INSERT INTO attachments(id,entity_type,entity_id,name,media_type,size,sha256,data,created_at) VALUES(?,?,?,?,?,?,?,?,?)", app.NewID(), "people", person, "secret-9481.txt", "text/plain", int64(len(email)), sqlite.Checksum([]byte(email)), []byte(email), app.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.RedactPerson(person, "NO"); err == nil {
		t.Fatal("missing deletion confirmation accepted")
	}
	if err := s.RedactPerson(person, "DELETE"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "after.zip")
	if err := portable.Export(s.DB, file); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	// The archive's compressed bytes may not contain every string verbatim; import
	// and inspect every persisted text field to establish the redaction invariant.
	_ = data
	dst := filepath.Join(t.TempDir(), "restored.db")
	if err := portable.ImportIntoEmpty(file, dst); err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range app.ExportTables {
		rows, err := db.Query("SELECT * FROM " + table)
		if err != nil {
			t.Fatal(err)
		}
		for _, row := range rows {
			for _, v := range row {
				val := app.String(v)
				if strings.Contains(val, "9481") || strings.Contains(val, email) {
					t.Fatalf("personal data remains in %s: %s", table, val)
				}
			}
		}
	}
}
