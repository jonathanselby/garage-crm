package app

import (
	"errors"
	"fmt"
	"github.com/jonathanselby/garage-crm/internal/sqlite"
	"math/big"
	"strconv"
	"strings"
	"time"
)

func (s *Service) Dashboard(brand string) (map[string]int64, error) {
	counters := map[string]int64{}
	for _, table := range []string{"people", "organisations", "leads", "opportunities", "campaigns", "tasks", "offers"} {
		q := "SELECT COUNT(*) AS count FROM " + table + " WHERE archived=0"
		args := []any{}
		if brand != "" && table != "people" && table != "organisations" {
			if table == "tasks" {
				q += " AND (brand_id=? OR brand_id IS NULL)"
			} else {
				q += " AND brand_id=?"
			}
			args = append(args, brand)
		}
		rows, err := s.DB.Query(q, args...)
		if err != nil {
			return nil, err
		}
		counters[table] = rows[0]["count"].(int64)
	}
	today := time.Now().Format("2006-01-02")
	q := "SELECT COUNT(*) count FROM tasks WHERE archived=0 AND status!='Done' AND due_date<=?"
	args := []any{today}
	if brand != "" {
		q += " AND (brand_id=? OR brand_id IS NULL)"
		args = append(args, brand)
	}
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	counters["due"] = rows[0]["count"].(int64)
	q = "SELECT COUNT(*) count FROM leads WHERE archived=0 AND status='New'"
	args = nil
	if brand != "" {
		q += " AND brand_id=?"
		args = append(args, brand)
	}
	rows, err = s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	counters["newleads"] = rows[0]["count"].(int64)
	return counters, nil
}
func (s *Service) MyDay(brand string) ([]sqlite.Row, error) {
	sql := "SELECT id,title,status,priority,due_date,brand_id FROM tasks WHERE archived=0 AND status!='Done'"
	args := []any{}
	if brand != "" {
		sql += " AND (brand_id=? OR brand_id IS NULL)"
		args = append(args, brand)
	}
	sql += " ORDER BY CASE WHEN due_date IS NULL THEN 1 ELSE 0 END, due_date ASC, CASE priority WHEN 'Urgent' THEN 0 WHEN 'High' THEN 1 ELSE 2 END LIMIT 15"
	return s.DB.Query(sql, args...)
}
func (s *Service) ConvertLead(id string, revision int64, makeOpportunity bool) (string, error) {
	var result string
	err := s.DB.Tx(func(tx *sqlite.Session) error {
		leads, err := tx.Query("SELECT * FROM leads WHERE id=? AND archived=0", id)
		if err != nil {
			return err
		}
		if len(leads) == 0 {
			return errors.New("lead not found")
		}
		lead := leads[0]
		if String(lead["converted_opportunity_id"]) != "" {
			result = String(lead["converted_opportunity_id"])
			return nil
		}
		if String(lead["status"]) == "Converted" {
			return nil
		}
		n, err := tx.Run("UPDATE leads SET status='Converted',revision=revision+1,updated_at=? WHERE id=? AND revision=?", Now(), id, revision)
		if err != nil {
			return err
		}
		if n != 1 {
			return errors.New("lead changed in another tab")
		}
		if makeOpportunity {
			result = NewID()
			currency := String(lead["currency"])
			if currency == "" {
				currency = "AUD"
			}
			_, err := tx.Run(`INSERT INTO opportunities(id,brand_id,person_id,organisation_id,lead_id,title,stage,amount_minor,currency,created_at,updated_at,primary_campaign_id)
   VALUES(?,?,?,?,?,?,'Discovery',?,?,?, ?, ?)`, result, lead["brand_id"], lead["person_id"], lead["organisation_id"], id, lead["title"], func() any {
				if lead["amount_minor"] == nil {
					return int64(0)
				}
				return lead["amount_minor"]
			}(), currency, Now(), Now(), lead["campaign_id"])
			if err != nil {
				return err
			}
			if err = tx.Exec("UPDATE leads SET converted_opportunity_id=? WHERE id=?", result, id); err != nil {
				return err
			}
			if err = tx.Exec("INSERT INTO stage_events(id,opportunity_id,from_stage,to_stage,changed_at) VALUES(?,?,?,'Discovery',?)", NewID(), result, "", Now()); err != nil {
				return err
			}
		}
		if err = tx.Exec("INSERT INTO lead_stage_events(id,lead_id,from_stage,to_stage,changed_at) VALUES(?,?,?,'Converted',?)", NewID(), id, lead["status"], Now()); err != nil {
			return err
		}
		if String(lead["person_id"]) != "" {
			err = tx.Exec(`INSERT OR IGNORE INTO brand_people(brand_id,person_id,lifecycle,source) VALUES(?,?,'Prospect',?)`, lead["brand_id"], lead["person_id"], lead["source"])
			if err != nil {
				return err
			}
		}
		return tx.Exec("INSERT INTO change_events(id,entity_type,entity_id,brand_id,action,summary,at) VALUES(?,?,?,?,?,?,?)", NewID(), "leads", id, lead["brand_id"], "convert", "lead converted", Now())
	})
	return result, err
}
func (s *Service) AddActivity(brand, person, organisation, lead, opportunity, campaign, channel, summary string, dueTask string, dueDate string) error {
	if strings.TrimSpace(summary) == "" {
		return errors.New("enter a note or outcome")
	}
	if len(summary) > 100000 {
		return errors.New("note too long")
	}
	if dueDate != "" {
		if _, err := time.Parse("2006-01-02", dueDate); err != nil {
			return errors.New("invalid follow-up date")
		}
	}
	return s.DB.Tx(func(tx *sqlite.Session) error {
		if err := tx.Exec(`INSERT INTO activities(id,brand_id,person_id,organisation_id,lead_id,opportunity_id,campaign_id,channel,summary,occurred_at,created_at)
  VALUES(?,?,?,?,?,?,?,?,?,?,?)`, NewID(), nullable(brand), nullable(person), nullable(organisation), nullable(lead), nullable(opportunity), nullable(campaign), channel, summary, Now(), Now()); err != nil {
			return err
		}
		if strings.TrimSpace(dueTask) != "" {
			if err := tx.Exec(`INSERT INTO tasks(id,brand_id,person_id,organisation_id,lead_id,opportunity_id,campaign_id,title,status,priority,due_date,created_at,updated_at)
   VALUES(?,?,?,?,?,?,?,?,'To Do','Medium',?,?,?)`, NewID(), nullable(brand), nullable(person), nullable(organisation), nullable(lead), nullable(opportunity), nullable(campaign), dueTask, nullable(dueDate), Now(), Now()); err != nil {
				return err
			}
		}
		return nil
	})
}
func (s *Service) History(name, id string) ([]sqlite.Row, error) {
	f := map[string]string{"people": "person_id", "organisations": "organisation_id", "leads": "lead_id", "opportunities": "opportunity_id", "campaigns": "campaign_id"}[name]
	if f == "" {
		return nil, nil
	}
	return s.DB.Query("SELECT * FROM activities WHERE "+f+"=? ORDER BY occurred_at DESC LIMIT 100", id)
}
func (s *Service) LinkContactToBrand(brand, person, lifecycle, consent string, optout bool) error {
	if lifecycle == "" {
		lifecycle = "Prospect"
	}
	if consent == "" {
		consent = "unknown"
	}
	if consent != "unknown" && consent != "yes" && consent != "no" {
		return errors.New("invalid consent")
	}
	return s.DB.Tx(func(tx *sqlite.Session) error {
		r, err := tx.Query("SELECT id FROM brands WHERE id=? AND archived=0", brand)
		if err != nil || len(r) != 1 {
			return errors.New("brand not found")
		}
		r, err = tx.Query("SELECT id FROM people WHERE id=? AND archived=0", person)
		if err != nil || len(r) != 1 {
			return errors.New("contact not found")
		}
		x := 0
		if optout {
			x = 1
		}
		return tx.Exec(`INSERT INTO brand_people(brand_id,person_id,lifecycle,consent,optout) VALUES(?,?,?,?,?)
  ON CONFLICT(brand_id,person_id) DO UPDATE SET lifecycle=excluded.lifecycle,consent=excluded.consent,optout=excluded.optout`, brand, person, lifecycle, consent, x)
	})
}
func (s *Service) ContactBrands(person string) ([]sqlite.Row, error) {
	return s.DB.Query(`SELECT bp.*,b.name AS brand_name FROM brand_people bp JOIN brands b ON b.id=bp.brand_id WHERE bp.person_id=? ORDER BY b.name`, person)
}
func (s *Service) Affiliations(person string) ([]sqlite.Row, error) {
	return s.DB.Query(`SELECT po.*,o.name AS organisation FROM person_organisations po JOIN organisations o ON o.id=po.organisation_id WHERE po.person_id=? ORDER BY o.name`, person)
}
func (s *Service) LinkAffiliation(person, organisation, role string) error {
	if person == "" || organisation == "" {
		return errors.New("choose both contact and organisation")
	}
	return s.DB.Exec(`INSERT INTO person_organisations(id,person_id,organisation_id,role) VALUES(?,?,?,?)`, NewID(), person, organisation, role)
}
func (s *Service) AddLine(opportunity, offer, description string, quantityMilli, priceMinor int64) error {
	if quantityMilli < 0 || priceMinor < 0 || quantityMilli > 100000000 || priceMinor > 100000000000 {
		return errors.New("amount outside supported range")
	}
	return s.DB.Tx(func(tx *sqlite.Session) error {
		o, err := tx.Query("SELECT brand_id,currency FROM opportunities WHERE id=?", opportunity)
		if err != nil || len(o) == 0 {
			return errors.New("opportunity not found")
		}
		if offer != "" {
			r, err := tx.Query("SELECT brand_id FROM offers WHERE id=?", offer)
			if err != nil || len(r) == 0 || String(r[0]["brand_id"]) != String(o[0]["brand_id"]) {
				return errors.New("offer must belong to opportunity brand")
			}
		}
		if description == "" && offer != "" {
			x, _ := tx.Query("SELECT name FROM offers WHERE id=?", offer)
			description = String(x[0]["name"])
		}
		if description == "" {
			return errors.New("enter an item description")
		}
		product := new(big.Int).Mul(big.NewInt(quantityMilli), big.NewInt(priceMinor))
		product.Add(product, big.NewInt(500))
		product.Div(product, big.NewInt(1000))
		if !product.IsInt64() {
			return errors.New("line item exceeds supported monetary range")
		}
		amount := product.Int64()
		if err := tx.Exec(`INSERT INTO opportunity_lines(id,opportunity_id,offer_id,description,quantity_milli,unit_price_minor,amount_minor,currency) VALUES(?,?,?,?,?,?,?,?)`, NewID(), opportunity, nullable(offer), description, quantityMilli, priceMinor, amount, o[0]["currency"]); err != nil {
			return err
		}
		return tx.Exec(`UPDATE opportunities SET amount_minor=(SELECT COALESCE(SUM(amount_minor),0) FROM opportunity_lines WHERE opportunity_id=?),revision=revision+1,updated_at=? WHERE id=?`, opportunity, Now(), opportunity)
	})
}
func (s *Service) Lines(opportunity string) ([]sqlite.Row, error) {
	return s.DB.Query("SELECT * FROM opportunity_lines WHERE opportunity_id=? ORDER BY rowid", opportunity)
}
func (s *Service) AddVariant(campaign, name, content string) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("variant name required")
	}
	return s.DB.Exec("INSERT INTO campaign_variants(id,campaign_id,name,content) VALUES(?,?,?,?)", NewID(), campaign, name, content)
}
func (s *Service) Variants(campaign string) ([]sqlite.Row, error) {
	return s.DB.Query("SELECT * FROM campaign_variants WHERE campaign_id=? ORDER BY name", campaign)
}
func (s *Service) CampaignMembers(campaign string) ([]sqlite.Row, error) {
	return s.DB.Query(`SELECT cm.*,p.global_dnc,bp.consent,bp.optout FROM campaign_members cm JOIN campaigns c ON c.id=cm.campaign_id LEFT JOIN aliases al ON al.old_id=cm.person_id AND al.entity_type='people' LEFT JOIN people p ON p.id=COALESCE(al.canonical_id,cm.person_id) LEFT JOIN brand_people bp ON bp.brand_id=c.brand_id AND bp.person_id=p.id WHERE cm.campaign_id=? ORDER BY cm.recipient_name`, campaign)
}
func (s *Service) ActivateCampaign(id string, revision int64) (int, error) {
	count := 0
	err := s.DB.Tx(func(tx *sqlite.Session) error {
		campaigns, err := tx.Query("SELECT * FROM campaigns WHERE id=? AND archived=0", id)
		if err != nil || len(campaigns) == 0 {
			return errors.New("campaign missing")
		}
		c := campaigns[0]
		if String(c["status"]) == "Active" {
			return nil
		}
		if String(c["status"]) == "Completed" || String(c["status"]) == "Cancelled" {
			return errors.New("cannot activate closed campaign")
		}
		q := `SELECT p.id,p.name,p.email FROM brand_people bp JOIN people p ON p.id=bp.person_id WHERE bp.brand_id=? AND p.archived=0 AND p.global_dnc=0 AND bp.optout=0 AND bp.consent='yes'`
		args := []any{c["brand_id"]}
		if text := String(c["audience_filter"]); text != "" {
			q += " AND (p.name LIKE ? OR p.tags LIKE ? OR bp.tags LIKE ?)"
			for i := 0; i < 3; i++ {
				args = append(args, "%"+text+"%")
			}
		}
		contacts, err := tx.Query(q, args...)
		if err != nil {
			return err
		}
		for _, p := range contacts {
			if err := tx.Exec(`INSERT OR IGNORE INTO campaign_members(id,campaign_id,person_id,recipient_name,recipient_email,added_at) VALUES(?,?,?,?,?,?)`, NewID(), id, p["id"], p["name"], p["email"], Now()); err != nil {
				return err
			}
			count++
		}
		n, err := tx.Run("UPDATE campaigns SET status='Active',revision=revision+1,updated_at=? WHERE id=? AND revision=?", Now(), id, revision)
		if err != nil {
			return err
		}
		if n != 1 {
			return errors.New("campaign changed in another tab")
		}
		return tx.Exec("INSERT INTO change_events(id,entity_type,entity_id,brand_id,action,summary,at) VALUES(?,?,?,?,?,?,?)", NewID(), "campaigns", id, c["brand_id"], "activate", fmt.Sprintf("snapshot activated (%d eligible)", len(contacts)), Now())
	})
	return count, err
}
func (s *Service) RecordOutcome(campaign, person, status, note string) error {
	allowed := []string{"Not contacted", "Contacted", "Replied", "Interested", "Converted", "Opted out", "Failed"}
	ok := false
	for _, v := range allowed {
		if v == status {
			ok = true
		}
	}
	if !ok {
		return errors.New("invalid outcome")
	}
	return s.DB.Tx(func(tx *sqlite.Session) error {
		n, err := tx.Run("UPDATE campaign_members SET status=?,note=? WHERE campaign_id=? AND person_id=?", status, note, campaign, person)
		if err != nil {
			return err
		}
		if n != 1 {
			return errors.New("campaign recipient not found")
		}
		if status == "Opted out" {
			x, err := tx.Query("SELECT brand_id FROM campaigns WHERE id=?", campaign)
			if err != nil || len(x) == 0 {
				return errors.New("campaign not found")
			}
			alias, err := tx.Query("SELECT canonical_id FROM aliases WHERE old_id=? AND entity_type='people'", person)
			if err != nil {
				return err
			}
			current := person
			if len(alias) > 0 {
				current = String(alias[0]["canonical_id"])
			}
			if err = tx.Exec("UPDATE brand_people SET optout=1,consent='no' WHERE brand_id=? AND person_id=?", x[0]["brand_id"], current); err != nil {
				return err
			}
		}
		return tx.Exec("INSERT INTO campaign_outcomes(id,campaign_id,person_id,status,note,at) VALUES(?,?,?,?,?,?)", NewID(), campaign, person, status, note, Now())
	})
}
func (s *Service) RecipientCSV(campaign string) (string, error) {
	rows, err := s.DB.Query(`SELECT p.name AS recipient_name,p.email FROM campaign_members cm JOIN campaigns c ON c.id=cm.campaign_id LEFT JOIN aliases al ON al.old_id=cm.person_id AND al.entity_type='people' JOIN people p ON p.id=COALESCE(al.canonical_id,cm.person_id) JOIN brand_people bp ON bp.brand_id=c.brand_id AND bp.person_id=p.id WHERE cm.campaign_id=? AND p.archived=0 AND p.global_dnc=0 AND bp.optout=0 AND bp.consent='yes' ORDER BY p.name`, campaign)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.WriteString("Name,Email\n")
	for _, r := range rows {
		b.WriteString(csvSafe(String(r["recipient_name"])) + "," + csvSafe(String(r["email"])) + "\n")
	}
	return b.String(), nil
}
func csvSafe(v string) string {
	v = strings.ReplaceAll(v, "\r", " ")
	v = strings.ReplaceAll(v, "\n", " ")
	v = strings.TrimSpace(v)
	if len(v) > 0 && strings.ContainsRune("=+-@\t", rune(v[0])) {
		v = "'" + v
	}
	return "\"" + strings.ReplaceAll(v, "\"", "\"\"") + "\""
}
func (s *Service) LeadsToFollowUp(brand string) ([]sqlite.Row, error) {
	q := "SELECT id,title,status,created_at FROM leads WHERE archived=0 AND status='New'"
	args := []any{}
	if brand != "" {
		q += " AND brand_id=?"
		args = append(args, brand)
	}
	q += " ORDER BY created_at DESC LIMIT 8"
	return s.DB.Query(q, args...)
}
func (s *Service) OpportunityStages(brand string) ([]sqlite.Row, error) {
	q := "SELECT stage,COUNT(*) AS total,currency,SUM(amount_minor) AS amount FROM opportunities WHERE archived=0 AND stage NOT IN ('Won','Lost')"
	args := []any{}
	if brand != "" {
		q += " AND brand_id=?"
		args = append(args, brand)
	}
	q += " GROUP BY stage,currency ORDER BY stage,currency"
	return s.DB.Query(q, args...)
}
func (s *Service) TaskNext(id string) error {
	return s.DB.Tx(func(tx *sqlite.Session) error {
		r, err := tx.Query("SELECT * FROM tasks WHERE id=?", id)
		if err != nil || len(r) == 0 {
			return errors.New("task missing")
		}
		t := r[0]
		if String(t["status"]) != "Done" || String(t["recurrence"]) == "" || String(t["due_date"]) == "" {
			return nil
		}
		due, err := time.Parse("2006-01-02", String(t["due_date"]))
		if err != nil {
			return err
		}
		switch String(t["recurrence"]) {
		case "Daily":
			due = due.AddDate(0, 0, 1)
		case "Weekly":
			due = due.AddDate(0, 0, 7)
		case "Monthly":
			due = due.AddDate(0, 1, 0)
		}
		today := time.Now()
		for due.Before(time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)) {
			switch String(t["recurrence"]) {
			case "Daily":
				due = due.AddDate(0, 0, 1)
			case "Weekly":
				due = due.AddDate(0, 0, 7)
			case "Monthly":
				due = due.AddDate(0, 1, 0)
			}
		}
		series := String(t["series_id"])
		if series == "" {
			series = id
		}
		old, err := tx.Query("SELECT id FROM tasks WHERE series_id=? AND status!='Done' AND archived=0", series)
		if err != nil {
			return err
		}
		if len(old) != 0 {
			return nil
		}
		return tx.Exec(`INSERT INTO tasks(id,brand_id,person_id,organisation_id,lead_id,opportunity_id,campaign_id,title,description,status,priority,due_date,recurrence,series_id,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,'To Do',?,?,?,?,?,?)`, NewID(), t["brand_id"], t["person_id"], t["organisation_id"], t["lead_id"], t["opportunity_id"], t["campaign_id"], t["title"], t["description"], t["priority"], due.Format("2006-01-02"), t["recurrence"], series, Now(), Now())
	})
}
func Int(s string) int64 { n, _ := strconv.ParseInt(s, 10, 64); return n }
