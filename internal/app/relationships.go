package app

import (
	"errors"
	"fmt"
	"github.com/jonathanselby/garage-crm/internal/sqlite"
	"strings"
)

// MergePreview returns a conservative, non-mutating impact assessment.
func (s *Service) MergePreview(winner, loser string) (map[string]int64, error) {
	if winner == "" || loser == "" || winner == loser {
		return nil, errors.New("choose two distinct contacts")
	}
	a, err := s.One("people", winner)
	if err != nil {
		return nil, err
	}
	b, err := s.One("people", loser)
	if err != nil {
		return nil, err
	}
	if String(a["archived"]) != "0" || String(b["archived"]) != "0" {
		return nil, errors.New("both contacts must be active")
	}
	counts := map[string]int64{}
	for _, v := range []struct{ t, col string }{{"brand_people", "person_id"}, {"leads", "person_id"}, {"opportunities", "person_id"}, {"campaign_members", "person_id"}, {"activities", "person_id"}, {"tasks", "person_id"}, {"person_organisations", "person_id"}} {
		rows, err := s.DB.Query("SELECT COUNT(*) AS n FROM "+v.t+" WHERE "+v.col+"=?", loser)
		if err != nil {
			return nil, err
		}
		counts[v.t] = rows[0]["n"].(int64)
	}
	return counts, nil
}
func strictConsent(a, b string) string {
	if a == "no" || b == "no" {
		return "no"
	}
	if a == "unknown" || b == "unknown" {
		return "unknown"
	}
	if a == "yes" && b == "yes" {
		return "yes"
	}
	return "unknown"
}
func (s *Service) MergePeople(winner, loser string) error {
	if _, err := s.MergePreview(winner, loser); err != nil {
		return err
	}
	return s.DB.Tx(func(tx *sqlite.Session) error {
		records, err := tx.Query("SELECT * FROM people WHERE id IN (?,?) AND archived=0", winner, loser)
		if err != nil || len(records) != 2 {
			return errors.New("merge source changed")
		}
		byID := map[string]sqlite.Row{}
		for _, v := range records {
			byID[String(v["id"])] = v
		}
		a, b := byID[winner], byID[loser]
		if a == nil || b == nil {
			return errors.New("merge source not found")
		}
		dnc := String(a["global_dnc"]) == "1" || String(b["global_dnc"]) == "1"
		name, email, phone, notes := String(a["name"]), String(a["email"]), String(a["phone"]), String(a["notes"])
		if email == "" {
			email = String(b["email"])
		}
		if phone == "" {
			phone = String(b["phone"])
		}
		if String(b["notes"]) != "" && String(b["notes"]) != notes {
			if notes != "" {
				notes += "\n"
			}
			notes += "[Merged notes] " + String(b["notes"])
		}
		if err := tx.Exec("UPDATE people SET name=?,email=?,phone=?,notes=?,global_dnc=?,revision=revision+1,updated_at=? WHERE id=?", name, email, phone, notes, dnc, Now(), winner); err != nil {
			return err
		}
		relations, err := tx.Query("SELECT * FROM brand_people WHERE person_id=?", loser)
		if err != nil {
			return err
		}
		for _, rel := range relations {
			brand := String(rel["brand_id"])
			existing, err := tx.Query("SELECT consent,optout FROM brand_people WHERE brand_id=? AND person_id=?", brand, winner)
			if err != nil {
				return err
			}
			if len(existing) > 0 {
				consent := strictConsent(String(existing[0]["consent"]), String(rel["consent"]))
				optout := String(existing[0]["optout"]) == "1" || String(rel["optout"]) == "1"
				if err := tx.Exec("UPDATE brand_people SET consent=?,optout=? WHERE brand_id=? AND person_id=?", consent, optout, brand, winner); err != nil {
					return err
				}
			} else {
				if err := tx.Exec(`INSERT INTO brand_people(brand_id,person_id,lifecycle,consent,optout,tags,source) VALUES(?,?,?,?,?,?,?)`, brand, winner, rel["lifecycle"], rel["consent"], rel["optout"], rel["tags"], rel["source"]); err != nil {
					return err
				}
			}
		}
		if err := tx.Exec("DELETE FROM brand_people WHERE person_id=?", loser); err != nil {
			return err
		}
		// Re-parent live relationships. Historic campaign memberships and outcomes
		// intentionally continue to reference the original identity and snapshot.
		for _, table := range []string{"leads", "opportunities", "tasks", "activities"} {
			if err := tx.Exec("UPDATE "+table+" SET person_id=? WHERE person_id=?", winner, loser); err != nil {
				return err
			}
		}
		if err := tx.Exec("UPDATE person_emails SET person_id=? WHERE person_id=?", winner, loser); err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM person_emails WHERE person_id=?", loser); err != nil {
			return err
		}
		if err := tx.Exec("UPDATE person_phones SET person_id=? WHERE person_id=?", winner, loser); err != nil {
			return err
		}
		// The affiliation uniqueness constraint may discard exact duplicates.
		if err := tx.Exec("UPDATE OR IGNORE person_organisations SET person_id=? WHERE person_id=?", winner, loser); err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM person_organisations WHERE person_id=?", loser); err != nil {
			return err
		}
		if err := tx.Exec("UPDATE OR IGNORE offer_interests SET person_id=? WHERE person_id=?", winner, loser); err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM offer_interests WHERE person_id=?", loser); err != nil {
			return err
		}
		if err := tx.Exec("UPDATE OR IGNORE opportunity_participants SET person_id=? WHERE person_id=?", winner, loser); err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM opportunity_participants WHERE person_id=?", loser); err != nil {
			return err
		}
		if err := tx.Exec("UPDATE aliases SET canonical_id=? WHERE canonical_id=?", winner, loser); err != nil {
			return err
		}
		if err := tx.Exec("INSERT INTO aliases(old_id,canonical_id,entity_type,at) VALUES(?,?,'people',?)", loser, winner, Now()); err != nil {
			return err
		}
		if err := tx.Exec("UPDATE people SET archived=1,updated_at=?,revision=revision+1 WHERE id=?", Now(), loser); err != nil {
			return err
		}
		return tx.Exec("INSERT INTO change_events(id,entity_type,entity_id,action,summary,at) VALUES(?,?,?,?,?,?)", NewID(), "people", winner, "merge", fmt.Sprintf("merged contact %s", loser), Now())
	})
}
func (s *Service) ContactDetails(person string) (emails, phones, interests []sqlite.Row, err error) {
	emails, err = s.DB.Query("SELECT * FROM person_emails WHERE person_id=? ORDER BY email", person)
	if err != nil {
		return
	}
	phones, err = s.DB.Query("SELECT * FROM person_phones WHERE person_id=? ORDER BY phone", person)
	if err != nil {
		return
	}
	interests, err = s.DB.Query("SELECT oi.*,o.name AS offer_name,b.name AS brand_name FROM offer_interests oi JOIN offers o ON o.id=oi.offer_id JOIN brands b ON b.id=oi.brand_id WHERE oi.person_id=? ORDER BY b.name,o.name", person)
	return
}
func (s *Service) AddContactDetail(person, kind, value, label string) error {
	kind = strings.ToLower(kind)
	if kind != "email" && kind != "phone" {
		return errors.New("unsupported detail type")
	}
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 1000 {
		return errors.New("invalid detail")
	}
	if _, err := s.One("people", person); err != nil {
		return err
	}
	table, col := "person_emails", "email"
	if kind == "phone" {
		table, col = "person_phones", "phone"
	}
	return s.DB.Exec("INSERT INTO "+table+"(id,person_id,"+col+",kind) VALUES(?,?,?,?)", NewID(), person, value, label)
}
func (s *Service) SetOfferInterest(person, offer, status string) error {
	if status == "" || len(status) > 100 {
		return errors.New("invalid product relationship status")
	}
	rows, err := s.DB.Query("SELECT brand_id FROM offers WHERE id=? AND archived=0", offer)
	if err != nil || len(rows) == 0 {
		return errors.New("offer not found")
	}
	brand := rows[0]["brand_id"]
	return s.DB.Tx(func(tx *sqlite.Session) error {
		people, err := tx.Query("SELECT id FROM people WHERE id=? AND archived=0", person)
		if err != nil || len(people) != 1 {
			return errors.New("contact not found")
		}
		if err := tx.Exec("INSERT OR IGNORE INTO brand_people(brand_id,person_id) VALUES(?,?)", brand, person); err != nil {
			return err
		}
		return tx.Exec(`INSERT INTO offer_interests(id,brand_id,person_id,offer_id,status) VALUES(?,?,?,?,?) ON CONFLICT(person_id,offer_id) DO UPDATE SET status=excluded.status`, NewID(), brand, person, offer, status)
	})
}

// RedactPerson retains non-identifying FK tombstones. It cannot erase old
// backups, already exported data, or forensic SSD remnants.
func (s *Service) RedactPerson(person, confirmation string) error {
	if confirmation != "DELETE" {
		return errors.New("type DELETE to confirm permanent redaction")
	}
	if _, err := s.One("people", person); err != nil {
		return err
	}
	err := s.DB.Tx(func(tx *sqlite.Session) error {
		rows, err := tx.Query("SELECT id FROM people WHERE id=? AND archived=0", person)
		if err != nil || len(rows) == 0 {
			return errors.New("record not found or already redacted")
		}
		for _, statement := range []string{
			"UPDATE people SET name='[Deleted contact]',title='',email='',phone='',notes='',tags='',global_dnc=1,archived=1,revision=revision+1,updated_at=? WHERE id=?",
		} {
			if err := tx.Exec(statement, Now(), person); err != nil {
				return err
			}
		}
		for _, table := range []string{"person_emails", "person_phones", "person_organisations", "offer_interests"} {
			if err := tx.Exec("DELETE FROM "+table+" WHERE person_id=?", person); err != nil {
				return err
			}
		}
		if err := tx.Exec("UPDATE brand_people SET lifecycle='Redacted',consent='no',optout=1,tags='',source='' WHERE person_id=?", person); err != nil {
			return err
		}
		if err := tx.Exec("UPDATE campaign_members SET recipient_name='',recipient_email='',note='' WHERE person_id=?", person); err != nil {
			return err
		}
		if err := tx.Exec("UPDATE campaign_outcomes SET note='' WHERE person_id=?", person); err != nil {
			return err
		}
		if err := tx.Exec("UPDATE activities SET summary='[Redacted]' WHERE person_id=?", person); err != nil {
			return err
		}
		if err := tx.Exec("UPDATE leads SET title='[Redacted]',notes='',reason='',source='' WHERE person_id=?", person); err != nil {
			return err
		}
		if err := tx.Exec("UPDATE opportunities SET title='[Redacted]',notes='',reason='' WHERE person_id=?", person); err != nil {
			return err
		}
		if err := tx.Exec("UPDATE tasks SET title='[Redacted]',description='' WHERE person_id=?", person); err != nil {
			return err
		}
		if err := tx.Exec("UPDATE change_events SET summary='[Redacted]' WHERE entity_type='people' AND entity_id=?", person); err != nil {
			return err
		}
		for _, table := range []string{"people", "leads", "opportunities", "tasks", "activities"} {
			if table == "people" {
				if err := tx.Exec("DELETE FROM attachments WHERE entity_type='people' AND entity_id=?", person); err != nil {
					return err
				}
				continue
			}
			ids, err := tx.Query("SELECT id FROM "+table+" WHERE person_id=?", person)
			if err != nil {
				return err
			}
			for _, row := range ids {
				if err := tx.Exec("DELETE FROM attachments WHERE entity_type=? AND entity_id=?", table, row["id"]); err != nil {
					return err
				}
			}
		}
		return tx.Exec("INSERT INTO change_events(id,entity_type,entity_id,action,summary,at) VALUES(?,?,?,?,?,?)", NewID(), "people", person, "redact", "personal data redacted", Now())
	})
	if err != nil {
		return err
	}
	// Best-effort reduction in recoverable WAL/free pages, not forensic erasure.
	if err = s.DB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		return err
	}
	return nil
}
