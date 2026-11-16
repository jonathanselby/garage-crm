package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/jonathanselby/garage-crm/internal/sqlite"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Field struct {
	Key, Label, Kind string
	Values           []string
	Required         bool
}
type Entity struct {
	Name, Title, Label string
	Fields             []Field
	Scoped             bool
}

var Entities = []Entity{
	{"brands", "Brands", "name", []Field{{"name", "Brand name", "text", nil, true}, {"description", "Description", "textarea", nil, false}, {"currency", "Currency", "currency", nil, false}, {"colour", "Accent colour", "color", nil, false}}, false},
	{"offers", "Products & services", "name", []Field{{"name", "Name", "text", nil, true}, {"brand_id", "Brand", "brand", nil, true}, {"kind", "Offer type", "select", []string{"Software", "Service", "Labour", "App", "Physical", "Other"}, false}, {"code", "Code / SKU", "text", nil, false}, {"description", "Description", "textarea", nil, false}, {"price_minor", "Indicative price (minor units)", "number", nil, false}, {"currency", "Currency", "currency", nil, false}}, true},
	{"people", "Contacts", "name", []Field{{"name", "Full name", "text", nil, true}, {"title", "Role / title", "text", nil, false}, {"email", "Email", "email", nil, false}, {"phone", "Phone", "text", nil, false}, {"tags", "Tags", "text", nil, false}, {"notes", "Notes", "textarea", nil, false}, {"global_dnc", "Global do not contact", "bool", nil, false}}, false},
	{"organisations", "Organisations", "name", []Field{{"name", "Organisation name", "text", nil, true}, {"website", "Website", "text", nil, false}, {"email", "Email", "email", nil, false}, {"phone", "Phone", "text", nil, false}, {"address", "Address", "textarea", nil, false}, {"tags", "Tags", "text", nil, false}, {"notes", "Notes", "textarea", nil, false}}, false},
	{"leads", "Leads", "title", []Field{{"title", "Lead / prospect", "text", nil, true}, {"brand_id", "Brand", "brand", nil, true}, {"person_id", "Contact", "person", nil, false}, {"organisation_id", "Organisation", "organisation", nil, false}, {"status", "Stage", "select", []string{"New", "Contacted", "Qualified", "Nurture", "Disqualified", "Converted"}, false}, {"priority", "Priority", "select", []string{"Low", "Medium", "High", "Urgent"}, false}, {"source", "Source", "text", nil, false}, {"offer_id", "Offer", "offer", nil, false}, {"campaign_id", "Campaign", "campaign", nil, false}, {"amount_minor", "Potential value (minor units)", "number", nil, false}, {"currency", "Currency", "currency", nil, false}, {"due_date", "Next date", "date", nil, false}, {"notes", "Qualification notes", "textarea", nil, false}, {"reason", "Reason", "text", nil, false}}, true},
	{"opportunities", "Opportunities", "title", []Field{{"title", "Opportunity name", "text", nil, true}, {"brand_id", "Brand", "brand", nil, true}, {"person_id", "Contact", "person", nil, false}, {"organisation_id", "Organisation", "organisation", nil, false}, {"stage", "Stage", "select", []string{"Discovery", "Qualified", "Proposal", "Negotiation", "Won", "Lost"}, false}, {"probability", "Probability %", "percent", nil, false}, {"amount_minor", "Value (minor units)", "number", nil, false}, {"currency", "Currency", "currency", nil, false}, {"expected_close", "Expected close", "date", nil, false}, {"primary_campaign_id", "Source campaign", "campaign", nil, false}, {"notes", "Notes", "textarea", nil, false}, {"reason", "Won/lost reason", "text", nil, false}}, true},
	{"campaigns", "Campaigns", "name", []Field{{"name", "Campaign name", "text", nil, true}, {"brand_id", "Brand", "brand", nil, true}, {"kind", "Channel / campaign type", "select", []string{"General", "Email (manual)", "LinkedIn (manual)", "Social", "Event", "Direct outreach", "Other"}, false}, {"status", "Status", "select", []string{"Draft", "Planned", "Active", "Completed", "Cancelled"}, false}, {"family_label", "Family label", "text", nil, false}, {"start_date", "Start", "date", nil, false}, {"end_date", "End", "date", nil, false}, {"budget_minor", "Budget (minor units)", "number", nil, false}, {"spend_minor", "Spend (minor units)", "number", nil, false}, {"currency", "Currency", "currency", nil, false}, {"audience_filter", "Segment contains (tag / name)", "text", nil, false}, {"message", "Template / content", "textarea", nil, false}, {"notes", "Notes", "textarea", nil, false}}, true},
	{"tasks", "Tasks", "title", []Field{{"title", "Task", "text", nil, true}, {"brand_id", "Brand (optional)", "brand", nil, false}, {"status", "Status", "select", []string{"Backlog", "To Do", "In Progress", "Blocked", "Done"}, false}, {"priority", "Priority", "select", []string{"Low", "Medium", "High", "Urgent"}, false}, {"due_date", "Due date", "date", nil, false}, {"person_id", "Contact", "person", nil, false}, {"lead_id", "Lead", "lead", nil, false}, {"opportunity_id", "Opportunity", "opportunity", nil, false}, {"campaign_id", "Campaign", "campaign", nil, false}, {"recurrence", "Repeat", "select", []string{"", "Daily", "Weekly", "Monthly"}, false}, {"description", "Notes", "textarea", nil, false}}, false},
}

func EntityByName(name string) (Entity, bool) {
	for _, e := range Entities {
		if e.Name == name {
			return e, true
		}
	}
	return Entity{}, false
}
func NewID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return hex.EncodeToString(b[:])
}
func Now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
func String(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func Get(rows []sqlite.Row, key string) string {
	if len(rows) == 0 {
		return ""
	}
	return String(rows[0][key])
}

type Service struct{ DB *sqlite.DB }

func (s *Service) Brands() ([]sqlite.Row, error) {
	return s.DB.Query("SELECT id,name,currency,colour FROM brands WHERE archived=0 ORDER BY name")
}
func (s *Service) Lists(name, brand, search string, limit int) ([]sqlite.Row, error) {
	e, ok := EntityByName(name)
	if !ok {
		return nil, errors.New("unknown section")
	}
	if limit < 1 || limit > 200 {
		limit = 100
	}
	sql := "SELECT * FROM " + e.Name + " WHERE archived=0"
	args := []any{}
	if e.Scoped && brand != "" {
		sql += " AND brand_id=?"
		args = append(args, brand)
	}
	if name == "tasks" && brand != "" {
		sql += " AND (brand_id=? OR brand_id IS NULL)"
		args = append(args, brand)
	}
	if search != "" {
		sql += " AND (" + e.Label + " LIKE ?)"
		args = append(args, "%"+search+"%")
	}
	col := "updated_at"
	if name == "tasks" {
		col = "CASE WHEN due_date IS NULL THEN 1 ELSE 0 END,due_date"
	}
	sql += " ORDER BY " + col + " DESC LIMIT ?"
	args = append(args, limit)
	return s.DB.Query(sql, args...)
}
func (s *Service) One(name, id string) (sqlite.Row, error) {
	e, ok := EntityByName(name)
	if !ok {
		return nil, errors.New("unknown section")
	}
	rows, err := s.DB.Query("SELECT * FROM "+e.Name+" WHERE id=?", id)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, errors.New("record not found")
	}
	return rows[0], nil
}
func (s *Service) validate(e Entity, vals map[string]string) (map[string]any, error) {
	out := map[string]any{}
	for _, f := range e.Fields {
		v := strings.TrimSpace(vals[f.Key])
		if f.Required && v == "" {
			return nil, fmt.Errorf("%s is required", f.Label)
		}
		if len(v) > 100000 {
			return nil, fmt.Errorf("%s too long", f.Label)
		}
		if f.Kind == "select" && v != "" {
			found := false
			for _, c := range f.Values {
				if c == v {
					found = true
				}
			}
			if !found {
				return nil, fmt.Errorf("invalid %s", f.Label)
			}
		}
		if f.Kind == "percent" && v != "" {
			num, err := strconv.Atoi(v)
			if err != nil || num < 0 || num > 100 {
				return nil, fmt.Errorf("invalid probability")
			}
		}
		if f.Kind == "date" && v != "" {
			if _, err := time.Parse("2006-01-02", v); err != nil {
				return nil, fmt.Errorf("invalid date for %s", f.Label)
			}
		}
		if f.Kind == "number" && v != "" {
			if _, err := strconv.ParseInt(v, 10, 64); err != nil {
				return nil, fmt.Errorf("invalid number for %s", f.Label)
			}
		}
		if f.Kind == "currency" && v != "" {
			if len(v) != 3 || strings.ToUpper(v) != v {
				return nil, fmt.Errorf("currency must be 3-letter uppercase ISO code")
			}
		}
		if f.Kind == "bool" {
			out[f.Key] = v == "1" || v == "on" || v == "true"
			continue
		}
		if v == "" {
			if f.Kind == "brand" || f.Kind == "person" || f.Kind == "organisation" || f.Kind == "lead" || f.Kind == "campaign" || f.Kind == "opportunity" || f.Kind == "offer" || f.Kind == "date" || f.Kind == "number" || f.Kind == "percent" {
				out[f.Key] = nil
			} else {
				out[f.Key] = ""
			}
			continue
		}
		if f.Kind == "number" || f.Kind == "percent" {
			n, _ := strconv.ParseInt(v, 10, 64)
			out[f.Key] = n
		} else {
			out[f.Key] = v
		}
	}
	return out, nil
}
func (s *Service) Save(name, id string, vals map[string]string, revision int64) (string, error) {
	e, ok := EntityByName(name)
	if !ok {
		return "", errors.New("unknown section")
	}
	v, err := s.validate(e, vals)
	if err != nil {
		return "", err
	}
	if name == "campaigns" && String(v["status"]) == "Active" {
		if revision == 0 {
			return "", errors.New("campaign activation requires an audience snapshot")
		}
		current, e := s.One(name, id)
		if e != nil || String(current["status"]) != "Active" {
			return "", errors.New("activate the audience snapshot before setting Active")
		}
	}
	if name == "leads" && String(v["status"]) == "Converted" {
		if revision == 0 {
			return "", errors.New("use the lead conversion workflow")
		}
		current, e := s.One(name, id)
		if e != nil || String(current["status"]) != "Converted" {
			return "", errors.New("use the lead conversion workflow")
		}
	}
	if e.Scoped && String(v["brand_id"]) == "" {
		return "", errors.New("choose or create a brand")
	}
	if name == "brands" && v["currency"] == nil {
		v["currency"] = "AUD"
	}
	if v["currency"] == nil {
		if name == "offers" || name == "leads" || name == "opportunities" || name == "campaigns" {
			v["currency"] = "AUD"
		}
	}
	if name == "opportunities" && v["probability"] == nil {
		v["probability"] = int64(20)
	}
	if name == "tasks" && v["status"] == nil {
		v["status"] = "To Do"
	}
	if name == "leads" && v["status"] == nil {
		v["status"] = "New"
	}
	if name == "opportunities" && v["stage"] == nil {
		v["stage"] = "Discovery"
	}
	if name == "campaigns" && v["status"] == nil {
		v["status"] = "Draft"
	}
	if (name == "leads" || name == "tasks") && v["priority"] == nil {
		v["priority"] = "Medium"
	}
	if name == "offers" && v["kind"] == nil {
		v["kind"] = "Service"
	}
	if name == "campaigns" && v["kind"] == nil {
		v["kind"] = "General"
	}
	if name == "opportunities" && v["amount_minor"] == nil {
		v["amount_minor"] = int64(0)
	}
	if name == "campaigns" {
		if v["budget_minor"] == nil {
			v["budget_minor"] = int64(0)
		}
		if v["spend_minor"] == nil {
			v["spend_minor"] = int64(0)
		}
	}
	if name == "offers" && v["price_minor"] == nil {
		v["price_minor"] = nil
	}
	if name == "brands" && v["colour"] == nil {
		v["colour"] = "#3e7ac4"
	}
	// Field names are from a fixed specification; values always bound.
	fields := make([]string, 0, len(v))
	for k := range v {
		fields = append(fields, k)
	}
	sort.Strings(fields)
	now := Now()
	if id == "" {
		id = NewID()
	}
	err = s.DB.Tx(func(tx *sqlite.Session) error {
		if err := s.validateReferences(tx, e, v); err != nil {
			return err
		}
		previousStage := ""
		if revision != 0 && (name == "opportunities" || name == "leads") {
			stageColumn := "status"
			if name == "opportunities" {
				stageColumn = "stage"
			}
			rows, err := tx.Query("SELECT "+stageColumn+" FROM "+name+" WHERE id=?", id)
			if err != nil {
				return err
			}
			if len(rows) == 0 {
				return errors.New("record not found")
			}
			previousStage = String(rows[0][stageColumn])
		}
		if revision == 0 {
			cols := append([]string{"id", "created_at", "updated_at"}, fields...)
			p := make([]string, len(cols))
			a := []any{id, now, now}
			for i := range p {
				p[i] = "?"
			}
			for _, k := range fields {
				a = append(a, v[k])
			}
			_, err := tx.Run("INSERT INTO "+name+"("+strings.Join(cols, ",")+") VALUES("+strings.Join(p, ",")+")", a...)
			if err != nil {
				return err
			}
		} else {
			sets := []string{}
			a := []any{}
			for _, k := range fields {
				sets = append(sets, k+"=?")
				a = append(a, v[k])
			}
			sets = append(sets, "updated_at=?", "revision=revision+1")
			a = append(a, now, id, revision)
			n, err := tx.Run("UPDATE "+name+" SET "+strings.Join(sets, ",")+" WHERE id=? AND revision=? AND archived=0", a...)
			if err != nil {
				return err
			}
			if n != 1 {
				return errors.New("record changed in another tab; reload before saving")
			}
		}
		action := "create"
		if revision != 0 {
			action = "update"
		}
		if revision != 0 && name == "opportunities" && previousStage != String(v["stage"]) {
			if err := tx.Exec("INSERT INTO stage_events(id,opportunity_id,from_stage,to_stage,changed_at) VALUES(?,?,?,?,?)", NewID(), id, previousStage, String(v["stage"]), now); err != nil {
				return err
			}
		}
		if revision != 0 && name == "leads" && previousStage != String(v["status"]) {
			if err := tx.Exec("INSERT INTO lead_stage_events(id,lead_id,from_stage,to_stage,changed_at) VALUES(?,?,?,?,?)", NewID(), id, previousStage, String(v["status"]), now); err != nil {
				return err
			}
		}
		return tx.Exec("INSERT INTO change_events(id,entity_type,entity_id,brand_id,action,summary,at) VALUES(?,?,?,?,?,?,?)", NewID(), name, id, nullable(String(v["brand_id"])), action, fmt.Sprintf("%s %s", action, name), now)
	})
	return id, err
}
func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
func (s *Service) validateReferences(tx *sqlite.Session, e Entity, v map[string]any) error {
	ids := []struct{ field, table string }{{"brand_id", "brands"}, {"person_id", "people"}, {"organisation_id", "organisations"}, {"offer_id", "offers"}, {"campaign_id", "campaigns"}, {"primary_campaign_id", "campaigns"}, {"lead_id", "leads"}, {"opportunity_id", "opportunities"}}
	brand := String(v["brand_id"])
	for _, r := range ids {
		if id := String(v[r.field]); id != "" {
			rows, err := tx.Query("SELECT id"+func() string {
				if r.table == "offers" || r.table == "campaigns" || r.table == "leads" || r.table == "opportunities" {
					return ",brand_id"
				}
				return ""
			}()+" FROM "+r.table+" WHERE id=? AND archived=0", id)
			if err != nil || len(rows) == 0 {
				return fmt.Errorf("invalid reference: %s", r.field)
			}
			if brand != "" && rows[0]["brand_id"] != nil && String(rows[0]["brand_id"]) != brand {
				return fmt.Errorf("%s belongs to a different brand", r.field)
			}
		}
	}
	return nil
}
func (s *Service) SetStatus(name, id, status string, revision int64) error {
	col := "status"
	allowed := map[string][]string{"leads": {"New", "Contacted", "Qualified", "Nurture", "Disqualified", "Converted"}, "campaigns": {"Draft", "Planned", "Active", "Completed", "Cancelled"}, "tasks": {"Backlog", "To Do", "In Progress", "Blocked", "Done"}, "opportunities": {"Discovery", "Qualified", "Proposal", "Negotiation", "Won", "Lost"}}
	if name == "opportunities" {
		col = "stage"
	}
	vals, ok := allowed[name]
	if !ok {
		return errors.New("invalid entity")
	}
	valid := false
	for _, x := range vals {
		if x == status {
			valid = true
		}
	}
	if !valid {
		return errors.New("invalid status")
	}
	if (name == "campaigns" && status == "Active") || (name == "leads" && status == "Converted") {
		return errors.New("use the dedicated conversion/activation workflow")
	}
	return s.DB.Tx(func(tx *sqlite.Session) error {
		old, err := tx.Query("SELECT "+col+",brand_id FROM "+name+" WHERE id=? AND archived=0", id)
		if err != nil || len(old) == 0 {
			return errors.New("record not found")
		}
		before := String(old[0][col])
		n, err := tx.Run("UPDATE "+name+" SET "+col+"=?,revision=revision+1,updated_at=? WHERE id=? AND revision=? AND archived=0", status, Now(), id, revision)
		if err != nil {
			return err
		}
		if n != 1 {
			return errors.New("record changed in another tab; reload before saving")
		}
		if status == before {
			return nil
		}
		if name == "opportunities" {
			if err = tx.Exec("INSERT INTO stage_events(id,opportunity_id,from_stage,to_stage,changed_at) VALUES(?,?,?,?,?)", NewID(), id, before, status, Now()); err != nil {
				return err
			}
		}
		if name == "leads" {
			if err = tx.Exec("INSERT INTO lead_stage_events(id,lead_id,from_stage,to_stage,changed_at) VALUES(?,?,?,?,?)", NewID(), id, before, status, Now()); err != nil {
				return err
			}
		}
		return tx.Exec("INSERT INTO change_events(id,entity_type,entity_id,brand_id,action,summary,at) VALUES(?,?,?,?,?,?,?)", NewID(), name, id, nullable(String(old[0]["brand_id"])), "status", before+" → "+status, Now())
	})
}
func (s *Service) Archive(name, id string, revision int64) error {
	e, ok := EntityByName(name)
	if !ok {
		return errors.New("invalid section")
	}
	_ = e
	return s.DB.Tx(func(tx *sqlite.Session) error {
		n, err := tx.Run("UPDATE "+name+" SET archived=1,revision=revision+1,updated_at=? WHERE id=? AND revision=?", Now(), id, revision)
		if err != nil {
			return err
		}
		if n != 1 {
			return errors.New("record changed or not found")
		}
		return tx.Exec("INSERT INTO change_events(id,entity_type,entity_id,action,summary,at) VALUES(?,?,?,?,?,?)", NewID(), name, id, "archive", "archived", Now())
	})
}
