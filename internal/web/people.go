package web

import (
	"net/http"
	"net/url"
)

func (s *Server) contactDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	person := r.FormValue("person_id")
	if err := s.Service.AddContactDetail(person, r.FormValue("kind"), r.FormValue("value"), r.FormValue("label")); err != nil {
		s.errView(w, r, err)
		return
	}
	s.redirect(w, r, "/record?type=people&id="+url.QueryEscape(person))
}
func (s *Server) offerInterest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	person := r.FormValue("person_id")
	if err := s.Service.SetOfferInterest(person, r.FormValue("offer_id"), r.FormValue("status")); err != nil {
		s.errView(w, r, err)
		return
	}
	s.redirect(w, r, "/record?type=people&id="+url.QueryEscape(person))
}
func (s *Server) merge(w http.ResponseWriter, r *http.Request) {
	winner, loser := r.FormValue("winner"), r.FormValue("loser")
	if r.Method == "GET" {
		if _, err := s.Service.MergePreview(winner, loser); err != nil {
			s.errView(w, r, err)
			return
		}
		counts, _ := s.Service.MergePreview(winner, loser)
		v := s.base("merge", "Confirm contact merge", "people", "")
		v.MergeCounts = counts
		v.ID = winner
		v.Notice = loser
		s.render(w, v)
		return
	}
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	if r.FormValue("confirm") != "MERGE" {
		s.errView(w, r, appError("type MERGE to confirm"))
		return
	}
	if err := s.Service.MergePeople(winner, loser); err != nil {
		s.errView(w, r, err)
		return
	}
	s.redirect(w, r, "/record?type=people&id="+url.QueryEscape(winner))
}
func (s *Server) redact(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method", 405)
		return
	}
	person := r.FormValue("person_id")
	if err := s.Service.RedactPerson(person, r.FormValue("confirmation")); err != nil {
		s.errView(w, r, err)
		return
	}
	s.redirect(w, r, "/list?type=people")
}

type appError string

func (e appError) Error() string { return string(e) }
