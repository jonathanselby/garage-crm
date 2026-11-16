package app

import (
	"fmt"
	"github.com/jonathanselby/garage-crm/internal/sqlite"
	"strings"
)

const schemaVersion = 1
const schemaSQL = `CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS brands(id TEXT PRIMARY KEY,name TEXT NOT NULL,description TEXT NOT NULL DEFAULT '',currency TEXT NOT NULL DEFAULT 'AUD',colour TEXT NOT NULL DEFAULT '#3e7ac4',archived INTEGER NOT NULL DEFAULT 0,revision INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS offers(id TEXT PRIMARY KEY,brand_id TEXT NOT NULL REFERENCES brands(id),name TEXT NOT NULL,kind TEXT NOT NULL DEFAULT 'service',code TEXT NOT NULL DEFAULT '',description TEXT NOT NULL DEFAULT '',price_minor INTEGER, currency TEXT NOT NULL DEFAULT 'AUD',archived INTEGER NOT NULL DEFAULT 0,revision INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS people(id TEXT PRIMARY KEY,name TEXT NOT NULL,title TEXT NOT NULL DEFAULT '',email TEXT NOT NULL DEFAULT '',phone TEXT NOT NULL DEFAULT '',notes TEXT NOT NULL DEFAULT '',tags TEXT NOT NULL DEFAULT '',global_dnc INTEGER NOT NULL DEFAULT 0,archived INTEGER NOT NULL DEFAULT 0,revision INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS person_emails(id TEXT PRIMARY KEY,person_id TEXT NOT NULL REFERENCES people(id),email TEXT NOT NULL,kind TEXT NOT NULL DEFAULT 'other');
CREATE TABLE IF NOT EXISTS person_phones(id TEXT PRIMARY KEY,person_id TEXT NOT NULL REFERENCES people(id),phone TEXT NOT NULL,kind TEXT NOT NULL DEFAULT 'other');
CREATE TABLE IF NOT EXISTS organisations(id TEXT PRIMARY KEY,name TEXT NOT NULL,website TEXT NOT NULL DEFAULT '',email TEXT NOT NULL DEFAULT '',phone TEXT NOT NULL DEFAULT '',address TEXT NOT NULL DEFAULT '',notes TEXT NOT NULL DEFAULT '',tags TEXT NOT NULL DEFAULT '',parent_id TEXT REFERENCES organisations(id),archived INTEGER NOT NULL DEFAULT 0,revision INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS person_organisations(id TEXT PRIMARY KEY,person_id TEXT NOT NULL REFERENCES people(id),organisation_id TEXT NOT NULL REFERENCES organisations(id),role TEXT NOT NULL DEFAULT '',started TEXT,ended TEXT,UNIQUE(person_id,organisation_id,role,started));
CREATE TABLE IF NOT EXISTS brand_people(brand_id TEXT NOT NULL REFERENCES brands(id),person_id TEXT NOT NULL REFERENCES people(id),lifecycle TEXT NOT NULL DEFAULT 'Prospect',consent TEXT NOT NULL DEFAULT 'unknown',optout INTEGER NOT NULL DEFAULT 0,tags TEXT NOT NULL DEFAULT '',source TEXT NOT NULL DEFAULT '',PRIMARY KEY(brand_id,person_id));
CREATE TABLE IF NOT EXISTS offer_interests(id TEXT PRIMARY KEY,brand_id TEXT NOT NULL REFERENCES brands(id),person_id TEXT NOT NULL REFERENCES people(id),offer_id TEXT NOT NULL REFERENCES offers(id),status TEXT NOT NULL DEFAULT 'Interested',UNIQUE(person_id,offer_id));
CREATE TABLE IF NOT EXISTS pipelines(id TEXT PRIMARY KEY,brand_id TEXT NOT NULL REFERENCES brands(id),name TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS pipeline_stages(id TEXT PRIMARY KEY,pipeline_id TEXT NOT NULL REFERENCES pipelines(id),name TEXT NOT NULL,position INTEGER NOT NULL,probability INTEGER NOT NULL DEFAULT 0,UNIQUE(pipeline_id,name));
CREATE TABLE IF NOT EXISTS leads(id TEXT PRIMARY KEY,brand_id TEXT NOT NULL REFERENCES brands(id),person_id TEXT REFERENCES people(id),organisation_id TEXT REFERENCES organisations(id),title TEXT NOT NULL,source TEXT NOT NULL DEFAULT '',campaign_id TEXT REFERENCES campaigns(id),offer_id TEXT REFERENCES offers(id),status TEXT NOT NULL DEFAULT 'New',priority TEXT NOT NULL DEFAULT 'Medium',score INTEGER,amount_minor INTEGER,currency TEXT NOT NULL DEFAULT 'AUD',due_date TEXT,reason TEXT NOT NULL DEFAULT '',notes TEXT NOT NULL DEFAULT '',converted_opportunity_id TEXT,archived INTEGER NOT NULL DEFAULT 0,revision INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS lead_offers(lead_id TEXT NOT NULL REFERENCES leads(id),offer_id TEXT NOT NULL REFERENCES offers(id),PRIMARY KEY(lead_id,offer_id));
CREATE TABLE IF NOT EXISTS lead_stage_events(id TEXT PRIMARY KEY,lead_id TEXT NOT NULL REFERENCES leads(id),from_stage TEXT NOT NULL,to_stage TEXT NOT NULL,changed_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS opportunities(id TEXT PRIMARY KEY,brand_id TEXT NOT NULL REFERENCES brands(id),person_id TEXT REFERENCES people(id),organisation_id TEXT REFERENCES organisations(id),lead_id TEXT REFERENCES leads(id),title TEXT NOT NULL,stage TEXT NOT NULL DEFAULT 'Discovery',pipeline_id TEXT REFERENCES pipelines(id),stage_id TEXT REFERENCES pipeline_stages(id),currency TEXT NOT NULL DEFAULT 'AUD',amount_minor INTEGER NOT NULL DEFAULT 0,probability INTEGER NOT NULL DEFAULT 20,expected_close TEXT,primary_campaign_id TEXT REFERENCES campaigns(id),reason TEXT NOT NULL DEFAULT '',notes TEXT NOT NULL DEFAULT '',archived INTEGER NOT NULL DEFAULT 0,revision INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS opportunity_lines(id TEXT PRIMARY KEY,opportunity_id TEXT NOT NULL REFERENCES opportunities(id),offer_id TEXT REFERENCES offers(id),description TEXT NOT NULL,quantity_milli INTEGER NOT NULL DEFAULT 1000,unit_price_minor INTEGER NOT NULL DEFAULT 0,amount_minor INTEGER NOT NULL DEFAULT 0,currency TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS opportunity_participants(opportunity_id TEXT NOT NULL REFERENCES opportunities(id),person_id TEXT NOT NULL REFERENCES people(id),role TEXT NOT NULL DEFAULT 'Other',PRIMARY KEY(opportunity_id,person_id));
CREATE TABLE IF NOT EXISTS stage_events(id TEXT PRIMARY KEY,opportunity_id TEXT NOT NULL REFERENCES opportunities(id),from_stage TEXT NOT NULL,to_stage TEXT NOT NULL,changed_at TEXT NOT NULL,reason TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS campaigns(id TEXT PRIMARY KEY,brand_id TEXT NOT NULL REFERENCES brands(id),name TEXT NOT NULL,kind TEXT NOT NULL DEFAULT 'General',status TEXT NOT NULL DEFAULT 'Draft',start_date TEXT,end_date TEXT,budget_minor INTEGER NOT NULL DEFAULT 0,spend_minor INTEGER NOT NULL DEFAULT 0,currency TEXT NOT NULL DEFAULT 'AUD',family_label TEXT NOT NULL DEFAULT '',audience_filter TEXT NOT NULL DEFAULT '',message TEXT NOT NULL DEFAULT '',notes TEXT NOT NULL DEFAULT '',archived INTEGER NOT NULL DEFAULT 0,revision INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS campaign_offers(campaign_id TEXT NOT NULL REFERENCES campaigns(id),offer_id TEXT NOT NULL REFERENCES offers(id),PRIMARY KEY(campaign_id,offer_id));
CREATE TABLE IF NOT EXISTS campaign_variants(id TEXT PRIMARY KEY,campaign_id TEXT NOT NULL REFERENCES campaigns(id),name TEXT NOT NULL,content TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS campaign_members(id TEXT PRIMARY KEY,campaign_id TEXT NOT NULL REFERENCES campaigns(id),person_id TEXT NOT NULL REFERENCES people(id),recipient_name TEXT NOT NULL,recipient_email TEXT NOT NULL DEFAULT '',variant_id TEXT REFERENCES campaign_variants(id),status TEXT NOT NULL DEFAULT 'Not contacted',note TEXT NOT NULL DEFAULT '',added_at TEXT NOT NULL,UNIQUE(campaign_id,person_id));
CREATE TABLE IF NOT EXISTS campaign_outcomes(id TEXT PRIMARY KEY,campaign_id TEXT NOT NULL REFERENCES campaigns(id),person_id TEXT NOT NULL REFERENCES people(id),status TEXT NOT NULL,note TEXT NOT NULL DEFAULT '',at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS tasks(id TEXT PRIMARY KEY,brand_id TEXT REFERENCES brands(id),person_id TEXT REFERENCES people(id),organisation_id TEXT REFERENCES organisations(id),lead_id TEXT REFERENCES leads(id),opportunity_id TEXT REFERENCES opportunities(id),campaign_id TEXT REFERENCES campaigns(id),title TEXT NOT NULL,description TEXT NOT NULL DEFAULT '',status TEXT NOT NULL DEFAULT 'To Do',priority TEXT NOT NULL DEFAULT 'Medium',due_date TEXT,position INTEGER NOT NULL DEFAULT 0,recurrence TEXT NOT NULL DEFAULT '',series_id TEXT,archived INTEGER NOT NULL DEFAULT 0,revision INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS task_checklist(id TEXT PRIMARY KEY,task_id TEXT NOT NULL REFERENCES tasks(id),label TEXT NOT NULL,done INTEGER NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS activities(id TEXT PRIMARY KEY,brand_id TEXT REFERENCES brands(id),person_id TEXT REFERENCES people(id),organisation_id TEXT REFERENCES organisations(id),lead_id TEXT REFERENCES leads(id),opportunity_id TEXT REFERENCES opportunities(id),campaign_id TEXT REFERENCES campaigns(id),channel TEXT NOT NULL DEFAULT 'Note',summary TEXT NOT NULL,occurred_at TEXT NOT NULL,created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS attachments(id TEXT PRIMARY KEY,entity_type TEXT NOT NULL,entity_id TEXT NOT NULL,name TEXT NOT NULL,media_type TEXT NOT NULL,size INTEGER NOT NULL CHECK(size>=0 AND size<=26214400),sha256 TEXT NOT NULL,data BLOB NOT NULL,created_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS custom_field_definitions(id TEXT PRIMARY KEY,entity_type TEXT NOT NULL,brand_id TEXT REFERENCES brands(id),name TEXT NOT NULL,kind TEXT NOT NULL,options TEXT NOT NULL DEFAULT '');
CREATE TABLE IF NOT EXISTS custom_field_values(definition_id TEXT NOT NULL REFERENCES custom_field_definitions(id),entity_id TEXT NOT NULL,value TEXT NOT NULL,PRIMARY KEY(definition_id,entity_id));
CREATE TABLE IF NOT EXISTS saved_views(id TEXT PRIMARY KEY,name TEXT NOT NULL,entity_type TEXT NOT NULL,brand_id TEXT REFERENCES brands(id),filter_json TEXT NOT NULL DEFAULT '{}');
CREATE TABLE IF NOT EXISTS aliases(old_id TEXT PRIMARY KEY,canonical_id TEXT NOT NULL,entity_type TEXT NOT NULL,at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS change_events(id TEXT PRIMARY KEY,entity_type TEXT NOT NULL,entity_id TEXT NOT NULL,brand_id TEXT,action TEXT NOT NULL,summary TEXT NOT NULL,at TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS idx_people_name ON people(name); CREATE INDEX IF NOT EXISTS idx_people_email ON people(email);
CREATE INDEX IF NOT EXISTS idx_organisations_name ON organisations(name);
CREATE INDEX IF NOT EXISTS idx_leads_brand ON leads(brand_id,status,created_at);
CREATE INDEX IF NOT EXISTS idx_opportunities_brand ON opportunities(brand_id,stage,created_at);
CREATE INDEX IF NOT EXISTS idx_campaigns_brand ON campaigns(brand_id,status);
CREATE INDEX IF NOT EXISTS idx_tasks_due ON tasks(status,due_date);
CREATE INDEX IF NOT EXISTS idx_tasks_brand ON tasks(brand_id,status);
CREATE INDEX IF NOT EXISTS idx_activities_person ON activities(person_id,occurred_at);
CREATE INDEX IF NOT EXISTS idx_campaign_members_campaign ON campaign_members(campaign_id);
CREATE INDEX IF NOT EXISTS idx_change_events_entity ON change_events(entity_type,entity_id,at);
`

func Migrate(db *sqlite.DB) error {
	if err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	v, err := db.Query("SELECT MAX(version) AS version FROM schema_migrations")
	if err != nil {
		return err
	}
	if len(v) != 0 && v[0]["version"] != nil && v[0]["version"].(int64) > schemaVersion {
		return fmt.Errorf("database schema newer than application")
	}
	if len(v) == 0 || v[0]["version"] == nil {
		return db.Exec("INSERT INTO schema_migrations(version,applied_at) VALUES(?,datetime('now'))", schemaVersion)
	}
	return nil
}

var ExportTables = []string{"settings", "brands", "offers", "people", "person_emails", "person_phones", "organisations", "person_organisations", "brand_people", "offer_interests", "pipelines", "pipeline_stages", "leads", "lead_offers", "lead_stage_events", "opportunities", "opportunity_lines", "opportunity_participants", "stage_events", "campaigns", "campaign_offers", "campaign_variants", "campaign_members", "campaign_outcomes", "tasks", "task_checklist", "activities", "attachments", "custom_field_definitions", "custom_field_values", "saved_views", "aliases", "change_events"}

func HasTable(name string) bool {
	for _, v := range ExportTables {
		if v == name {
			return true
		}
	}
	return false
}
func TableNames() string { return strings.Join(ExportTables, ", ") }
