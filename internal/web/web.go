package web

import (
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/jonathanselby/garage-crm/internal/app"
	"github.com/jonathanselby/garage-crm/internal/portable"
	"github.com/jonathanselby/garage-crm/internal/sqlite"
	"html/template"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed static/site.css static/mark.svg
var assets embed.FS

func token() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

type Option struct{ ID, Label string }
type FieldView struct {
	F       app.Field
	Value   string
	Options []Option
}
type View struct {
	Mode, Title, Subtitle, Section, Brand, Search, Error, Notice, CSRF               string
	BrandOptions                                                                     []Option
	Nav                                                                              []NavItem
	Stats                                                                            map[string]int64
	Entity                                                                           app.Entity
	Rows                                                                             []sqlite.Row
	Record                                                                           sqlite.Row
	Fields                                                                           []FieldView
	Tasks, Leads, Stages, History, Lines, Members, Relations, Affiliations, Variants []sqlite.Row
	Board                                                                            map[string][]sqlite.Row
	BoardStates                                                                      []string
	ID                                                                               string
	Options                                                                          map[string][]Option
	Attachments                                                                      []sqlite.Row
	Emails, Phones, Interests                                                        []sqlite.Row
	MergeCounts                                                                      map[string]int64

	Cooling []sqlite.Row
}
type NavItem struct{ Href, Title, Icon, Name string }
type Server struct {
	Service                  *app.Service
	Addr                     string
	Session, CSRF, Bootstrap string
	tpl                      *template.Template
	bootstrapMu              sync.Mutex
	bootstrapExpires         time.Time
}

func New(s *app.Service, addr string) *Server {
	t := template.Must(template.New("page").Funcs(template.FuncMap{"str": app.String, "eqs": func(x any, y string) bool { return app.String(x) == y }, "formatdate": func(x any) string {
		v := app.String(x)
		if len(v) >= 10 {
			return v[:10]
		}
		return v
	}, "short": func(x any) string {
		v := app.String(x)
		if len(v) > 110 {
			return v[:110] + "…"
		}
		return v
	}}).Parse(pageTemplate))
	return &Server{Service: s, Addr: addr, Session: token(), CSRF: token(), Bootstrap: token(), tpl: t, bootstrapExpires: time.Now().Add(5 * time.Minute)}
}
func (s *Server) BootstrapFile(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "garage-unlock-*.html")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err = os.Chmod(f.Name(), 0600); err != nil {
		return "", err
	}
	// Token is only in the local owner-readable HTML file, never a URL or argv.
	html := fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="utf-8"><title>Garage CRM</title></head><body><p>Opening Garage CRM…</p><form id="start" method="POST" action="http://%s/bootstrap"><input type="hidden" name="token" value="%s"></form><script>document.getElementById('start').submit()</script><noscript><button form="start">Open Garage CRM</button></noscript></body></html>`, template.HTMLEscapeString(s.Addr), template.HTMLEscapeString(s.Bootstrap))
	if _, err = f.WriteString(html); err != nil {
		return "", err
	}
	return f.Name(), f.Sync()
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	// Preserve the same-origin referrer on local form submissions.
	// no-referrer can produce Origin: null, rejected by our CSRF Origin check.
	w.Header().Set("Referrer-Policy", "same-origin")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; img-src 'self' data:; script-src 'none'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	if r.Host != s.Addr {
		http.Error(w, "invalid host", http.StatusForbidden)
		return
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && net.ParseIP(ip) != nil && !net.ParseIP(ip).IsLoopback() {
		http.Error(w, "loopback only", 403)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/static/") {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "static/site.css" && name != "static/mark.svg" {
			http.NotFound(w, r)
			return
		}
		b, err := assets.ReadFile(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if strings.HasSuffix(name, ".css") {
			w.Header().Set("Content-Type", "text/css; charset=utf-8")
		} else {
			w.Header().Set("Content-Type", "image/svg+xml")
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write(b)
		return
	}
	if r.URL.Path == "/bootstrap" {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", 405)
			return
		}
		if r.Header.Get("Origin") != "null" && r.Header.Get("Origin") != "" {
			http.Error(w, "invalid bootstrap origin", 403)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err = r.ParseForm(); err != nil {
			http.Error(w, "invalid bootstrap", 400)
			return
		}
		s.bootstrapMu.Lock()
		defer s.bootstrapMu.Unlock()
		if time.Now().After(s.bootstrapExpires) || s.Bootstrap == "" || subtle.ConstantTimeCompare([]byte(r.Form.Get("token")), []byte(s.Bootstrap)) != 1 {
			http.Error(w, "invalid bootstrap", 403)
			return
		}
		s.Bootstrap = ""
		http.SetCookie(w, &http.Cookie{Name: "garage_session", Value: s.Session, HttpOnly: true, SameSite: http.SameSiteLaxMode, Path: "/", MaxAge: 86400})
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	cookie, err := r.Cookie("garage_session")
	if err != nil || subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(s.Session)) != 1 {
		w.WriteHeader(401)
		io.WriteString(w, "Garage CRM is locked. Launch the private Garage CRM starter file created by the application.")
		return
	}
	if r.Method == "POST" {
		if r.Header.Get("Origin") != "http://"+s.Addr {
			http.Error(w, "cross-origin request refused", 403)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 26<<20)
		if err := r.ParseMultipartForm(26 << 20); err != nil {
			if err = r.ParseForm(); err != nil {
				http.Error(w, "invalid form", 400)
				return
			}
		}
		if subtle.ConstantTimeCompare([]byte(r.FormValue("_csrf")), []byte(s.CSRF)) != 1 {
			http.Error(w, "CSRF validation failed", 403)
			return
		}
	}
	switch r.URL.Path {
	case "/":
		s.dashboard(w, r)
	case "/list":
		s.list(w, r)
	case "/record":
		s.record(w, r)
	case "/save":
		s.save(w, r)
	case "/status":
		s.status(w, r)
	case "/archive":
		s.archive(w, r)
	case "/board":
		s.board(w, r)
	case "/reports":
		s.reports(w, r)
	case "/activity":
		s.activity(w, r)
	case "/convert":
		s.convert(w, r)
	case "/activate":
		s.activate(w, r)
	case "/outcome":
		s.outcome(w, r)
	case "/recipients":
		s.recipients(w, r)
	case "/brand-contact":
		s.brandContact(w, r)
	case "/affiliation":
		s.affiliation(w, r)
	case "/line":
		s.line(w, r)
	case "/variant":
		s.variant(w, r)
	case "/upload":
		s.upload(w, r)
	case "/attachment":
		s.attachment(w, r)
	case "/export":
		s.export(w, r)
	case "/merge":
		s.merge(w, r)
	case "/redact":
		s.redact(w, r)
	case "/contact-detail":
		s.contactDetail(w, r)
	case "/offer-interest":
		s.offerInterest(w, r)
	default:
		http.NotFound(w, r)
	}
}
func (s *Server) base(mode, title, rpath, brand string) View {
	brands, _ := s.Service.Brands()
	opts := []Option{{"", "All brands"}}
	for _, b := range brands {
		opts = append(opts, Option{app.String(b["id"]), app.String(b["name"])})
	}
	nav := []NavItem{{"/", "My Day", "◈", "home"}, {"/list?type=people", "Contacts", "♙", "people"}, {"/list?type=organisations", "Organisations", "▤", "organisations"}, {"/list?type=leads", "Leads", "◎", "leads"}, {"/list?type=opportunities", "Opportunities", "◇", "opportunities"}, {"/list?type=campaigns", "Campaigns", "◉", "campaigns"}, {"/board", "Tasks / Kanban", "▦", "tasks"}, {"/list?type=offers", "Products & services", "▧", "offers"}, {"/list?type=brands", "Brands", "▣", "brands"}, {"/reports", "Reports", "▥", "reports"}}
	return View{Mode: mode, Title: title, Brand: brand, CSRF: s.CSRF, BrandOptions: opts, Nav: nav, Section: rpath}
}
func (s *Server) render(w http.ResponseWriter, v View) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tpl.Execute(w, v); err != nil {
		log.Printf("render: %v", err)
	}
}
func (s *Server) brand(r *http.Request) string { return r.URL.Query().Get("brand") }
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method", 405)
		return
	}
	b := s.brand(r)
	v := s.base("dashboard", "Good to have you back", "home", b)
	v.Subtitle = "Your relationships, opportunities and next actions — all in one place."
	v.Stats, _ = s.Service.Dashboard(b)
	v.Tasks, _ = s.Service.MyDay(b)
	v.Leads, _ = s.Service.LeadsToFollowUp(b)
	var coolingErr error
	v.Cooling, coolingErr = s.Service.CoolingItems(b)
	if coolingErr != nil {
		log.Printf("cooling query: %v", coolingErr)
		http.Error(w, "Dashboard unavailable", http.StatusInternalServerError)
		return
	}
	if v.Stats != nil {
		v.Stats["cooling"] = 0
		if len(v.Cooling) != 0 {
			v.Stats["cooling"] = v.Cooling[0]["total_count"].(int64)
		}
	}
	s.render(w, v)
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method", 405)
		return
	}
	name := r.URL.Query().Get("type")
	e, ok := app.EntityByName(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	b := s.brand(r)
	v := s.base("list", e.Title, name, b)
	v.Entity = e
	v.Search = r.URL.Query().Get("q")
	v.Subtitle = "One place to manage every " + strings.ToLower(e.Title) + " record."
	rows, err := s.Service.Lists(name, b, v.Search, 150)
	if err != nil {
		v.Error = err.Error()
	}
	v.Rows = rows
	s.render(w, v)
}
func (s *Server) options(kind, brand string) []Option {
	entity := map[string]string{"brand": "brands", "person": "people", "organisation": "organisations", "lead": "leads", "opportunity": "opportunities", "campaign": "campaigns", "offer": "offers"}[kind]
	if entity == "" {
		return nil
	}
	list, _ := s.Service.Lists(entity, brand, "", 200)
	opts := []Option{{"", "— None / choose —"}}
	if kind == "brand" {
		list, _ = s.Service.Lists(entity, "", "", 200)
	}
	for _, item := range list {
		label := "name"
		if entity == "leads" || entity == "opportunities" {
			label = "title"
		}
		opts = append(opts, Option{app.String(item["id"]), app.String(item[label])})
	}
	return opts
}
func (s *Server) record(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method", 405)
		return
	}
	name := r.URL.Query().Get("type")
	e, ok := app.EntityByName(name)
	if !ok {
		http.NotFound(w, r)
		return
	}
	id := r.URL.Query().Get("id")
	mode := "form"
	if id != "" && r.URL.Query().Get("edit") != "1" {
		mode = "detail"
	}
	b := s.brand(r)
	v := s.base(mode, e.Title, name, b)
	v.Entity = e
	v.ID = id
	v.Options = map[string][]Option{}
	for _, kind := range []string{"brand", "person", "organisation", "offer", "lead", "campaign"} {
		v.Options[kind] = s.options(kind, b)
	}
	if id != "" {
		rec, err := s.Service.One(name, id)
		if err != nil {
			v.Error = err.Error()
		} else {
			v.Record = rec
			v.Attachments, _ = s.Service.DB.Query("SELECT id,name,size FROM attachments WHERE entity_type=? AND entity_id=? ORDER BY created_at DESC", name, id)
			v.Title = app.String(rec[e.Label])
			v.History, _ = s.Service.History(name, id)
			if name == "people" {
				v.Relations, _ = s.Service.ContactBrands(id)
				v.Affiliations, _ = s.Service.Affiliations(id)
				v.Emails, v.Phones, v.Interests, _ = s.Service.ContactDetails(id)
			}
			if name == "opportunities" {
				v.Lines, _ = s.Service.Lines(id)
			}
			if name == "campaigns" {
				v.Members, _ = s.Service.CampaignMembers(id)
				v.Variants, _ = s.Service.Variants(id)
			}
		}
	} else {
		v.Title = "New " + strings.TrimSuffix(e.Title, "s")
	}
	if mode == "form" {
		for _, f := range e.Fields {
			value := ""
			if id != "" && v.Record != nil {
				value = app.String(v.Record[f.Key])
			} else {
				if f.Key == "brand_id" {
					value = b
				}
				if f.Key == "currency" {
					value = "AUD"
				}
				if f.Key == "colour" {
					value = "#36759a"
				}
				if f.Key == "priority" {
					value = "Medium"
				}
				if f.Key == "status" && name == "tasks" {
					value = "To Do"
				}
			}
			options := []Option{}
			if f.Kind == "brand" || f.Kind == "person" || f.Kind == "organisation" || f.Kind == "lead" || f.Kind == "opportunity" || f.Kind == "offer" || f.Kind == "campaign" {
				options = s.options(f.Kind, b)
			}
			v.Fields = append(v.Fields, FieldView{F: f, Value: value, Options: options})
		}
	}
	s.render(w, v)
}
func (s *Server) save(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	typ := r.FormValue("type")
	id := r.FormValue("id")
	rev := app.Int(r.FormValue("rev"))
	e, ok := app.EntityByName(typ)
	if !ok {
		http.Error(w, "invalid type", 400)
		return
	}
	vals := map[string]string{}
	for _, f := range e.Fields {
		vals[f.Key] = r.FormValue(f.Key)
	}
	out, err := s.Service.Save(typ, id, vals, rev)
	if err != nil {
		s.errView(w, r, err)
		return
	}
	s.redirect(w, r, "/record?type="+typ+"&id="+url.QueryEscape(out))
}
func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	typ, id, status := r.FormValue("type"), r.FormValue("id"), r.FormValue("status")
	if err := s.Service.SetStatus(typ, id, status, app.Int(r.FormValue("rev"))); err != nil {
		s.errView(w, r, err)
		return
	}
	if typ == "tasks" && status == "Done" {
		if err := s.Service.TaskNext(id); err != nil {
			log.Printf("recurrence: %v", err)
		}
	}
	s.redirect(w, r, r.FormValue("return"))
}
func (s *Server) archive(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	typ, id := r.FormValue("type"), r.FormValue("id")
	if err := s.Service.Archive(typ, id, app.Int(r.FormValue("rev"))); err != nil {
		s.errView(w, r, err)
		return
	}
	s.redirect(w, r, "/list?type="+typ)
}
func (s *Server) board(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method", 405)
		return
	}
	b := s.brand(r)
	v := s.base("board", "Task board", "tasks", b)
	v.Subtitle = "Move the work forward. One next action at a time."
	v.Board = map[string][]sqlite.Row{}
	v.BoardStates = []string{"Backlog", "To Do", "In Progress", "Blocked", "Done"}
	rows, _ := s.Service.Lists("tasks", b, "", 200)
	for _, row := range rows {
		v.Board[app.String(row["status"])] = append(v.Board[app.String(row["status"])], row)
	}
	s.render(w, v)
}
func (s *Server) reports(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method", 405)
		return
	}
	b := s.brand(r)
	v := s.base("reports", "Business reports", "reports", b)
	v.Stats, _ = s.Service.Dashboard(b)
	v.Stages, _ = s.Service.OpportunityStages(b)
	s.render(w, v)
}
func (s *Server) activity(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	typ, id := r.FormValue("type"), r.FormValue("id")
	args := map[string]string{typ: id}
	err := s.Service.AddActivity(r.FormValue("brand_id"), args["people"], args["organisations"], args["leads"], args["opportunities"], args["campaigns"], r.FormValue("channel"), r.FormValue("summary"), r.FormValue("task_title"), r.FormValue("due_date"))
	if err != nil {
		s.errView(w, r, err)
		return
	}
	s.redirect(w, r, "/record?type="+typ+"&id="+url.QueryEscape(id))
}
func (s *Server) convert(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	id := r.FormValue("id")
	out, err := s.Service.ConvertLead(id, app.Int(r.FormValue("rev")), r.FormValue("make_opportunity") == "yes")
	if err != nil {
		s.errView(w, r, err)
		return
	}
	if out != "" {
		s.redirect(w, r, "/record?type=opportunities&id="+url.QueryEscape(out))
	} else {
		s.redirect(w, r, "/record?type=leads&id="+url.QueryEscape(id))
	}
}
func (s *Server) activate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	id := r.FormValue("id")
	_, err := s.Service.ActivateCampaign(id, app.Int(r.FormValue("rev")))
	if err != nil {
		s.errView(w, r, err)
		return
	}
	s.redirect(w, r, "/record?type=campaigns&id="+url.QueryEscape(id))
}
func (s *Server) outcome(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	id := r.FormValue("id")
	if err := s.Service.RecordOutcome(id, r.FormValue("person_id"), r.FormValue("status"), r.FormValue("note")); err != nil {
		s.errView(w, r, err)
		return
	}
	s.redirect(w, r, "/record?type=campaigns&id="+url.QueryEscape(id))
}
func (s *Server) recipients(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method", 405)
		return
	}
	out, err := s.Service.RecipientCSV(r.URL.Query().Get("id"))
	if err != nil {
		s.errView(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="garage-campaign-recipients.csv"`)
	w.Write([]byte(out))
}
func (s *Server) brandContact(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	person := r.FormValue("person_id")
	if err := s.Service.LinkContactToBrand(r.FormValue("brand_id"), person, r.FormValue("lifecycle"), r.FormValue("consent"), r.FormValue("optout") == "on"); err != nil {
		s.errView(w, r, err)
		return
	}
	s.redirect(w, r, "/record?type=people&id="+url.QueryEscape(person))
}
func (s *Server) affiliation(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	person := r.FormValue("person_id")
	if err := s.Service.LinkAffiliation(person, r.FormValue("organisation_id"), r.FormValue("role")); err != nil {
		s.errView(w, r, err)
		return
	}
	s.redirect(w, r, "/record?type=people&id="+url.QueryEscape(person))
}
func (s *Server) line(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	id := r.FormValue("id")
	if err := s.Service.AddLine(id, r.FormValue("offer_id"), r.FormValue("description"), app.Int(r.FormValue("quantity_milli")), app.Int(r.FormValue("unit_price_minor"))); err != nil {
		s.errView(w, r, err)
		return
	}
	s.redirect(w, r, "/record?type=opportunities&id="+url.QueryEscape(id))
}
func (s *Server) variant(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	id := r.FormValue("id")
	if err := s.Service.AddVariant(id, r.FormValue("name"), r.FormValue("content")); err != nil {
		s.errView(w, r, err)
		return
	}
	s.redirect(w, r, "/record?type=campaigns&id="+url.QueryEscape(id))
}
func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	typ, id := r.FormValue("type"), r.FormValue("id")
	if _, ok := app.EntityByName(typ); !ok {
		http.Error(w, "invalid type", 400)
		return
	}
	if _, err := s.Service.One(typ, id); err != nil {
		s.errView(w, r, err)
		return
	}
	f, header, err := r.FormFile("attachment")
	if err != nil {
		s.errView(w, r, err)
		return
	}
	defer f.Close()
	if header.Size > 25<<20 {
		s.errView(w, r, errors.New("attachment exceeds 25 MiB"))
		return
	}
	bytes, err := io.ReadAll(io.LimitReader(f, (25<<20)+1))
	if err != nil || len(bytes) > 25<<20 {
		s.errView(w, r, errors.New("attachment too large"))
		return
	}
	name := filepath.Base(header.Filename)
	if name == "." || name == "/" {
		name = "attachment"
	}
	err = s.Service.DB.Exec("INSERT INTO attachments(id,entity_type,entity_id,name,media_type,size,sha256,data,created_at) VALUES(?,?,?,?,?,?,?,?,?)", app.NewID(), typ, id, name, "application/octet-stream", int64(len(bytes)), sqlite.Checksum(bytes), bytes, app.Now())
	if err != nil {
		s.errView(w, r, err)
		return
	}
	s.redirect(w, r, "/record?type="+typ+"&id="+url.QueryEscape(id))
}
func (s *Server) attachment(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method", 405)
		return
	}
	row, err := s.Service.DB.Query("SELECT name,data,sha256 FROM attachments WHERE id=?", r.URL.Query().Get("id"))
	if err != nil || len(row) != 1 {
		http.NotFound(w, r)
		return
	}
	b, ok := row[0]["data"].([]byte)
	if !ok || sqlite.Checksum(b) != app.String(row[0]["sha256"]) {
		http.Error(w, "attachment damaged", 500)
		return
	}
	name := strings.Map(func(ch rune) rune {
		if ch < 32 || ch == 127 || ch == '"' || ch == '\\' {
			return -1
		}
		return ch
	}, app.String(row[0]["name"]))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	w.Write(b)
}
func (s *Server) export(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method", 405)
		return
	}
	tmp, err := os.CreateTemp("", "garage-webexport-*.zip")
	if err != nil {
		s.errView(w, r, err)
		return
	}
	path := tmp.Name()
	tmp.Close()
	os.Remove(path)
	defer os.Remove(path)
	if err = portable.Export(s.Service.DB, path); err != nil {
		s.errView(w, r, err)
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="garage-crm-portable.zip"`)
	w.Header().Set("Content-Type", "application/zip")
	http.ServeFile(w, r, path)
}
func (s *Server) errView(w http.ResponseWriter, r *http.Request, err error) {
	w.WriteHeader(http.StatusBadRequest)
	v := s.base("error", "That change wasn't saved", "error", "")
	v.Error = err.Error()
	s.render(w, v)
}
func (s *Server) redirect(w http.ResponseWriter, r *http.Request, target string) {
	if !strings.HasPrefix(target, "/") || strings.HasPrefix(target, "//") {
		target = "/"
	}
	http.Redirect(w, r, target, http.StatusSeeOther)
}
