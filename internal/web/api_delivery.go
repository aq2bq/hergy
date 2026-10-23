package web

import (
	"net/http"

	"github.com/aq2bq/hergy/internal/deliver"
)

// Deliverer is the reading behind the band's count and the sheet a person opens
// to hand a changed definition to running members. The app's activity reading
// implements it; a server built without one answers an empty list.
type Deliverer interface {
	// Delivery is the running members whose prompt differs from the one hergy
	// last handed them, by running team.
	Delivery() deliver.Answer
	// DeliveryCount is the band's number alone.
	DeliveryCount() int
	// Deliver hands the current prompt to the named members, refusing the whole
	// request if one is not a waiting, stale member.
	Deliver(names []string) (deliver.Answer, error)
}

// emptyDelivery is what a server with no deliverer answers: nothing stale, and
// a herdr that could be read.
func emptyDelivery() deliver.Answer {
	return deliver.Answer{Teams: []deliver.Team{}, Herdr: deliver.Note{OK: true}}
}

// apiDeliveryList answers GET /api/delivery.
func (s *Server) apiDeliveryList(w http.ResponseWriter, _ *http.Request) {
	answer := emptyDelivery()
	if s.deliver != nil {
		answer = s.deliver.Delivery()
	}
	writeJSON(w, http.StatusOK, answer)
}

// apiDeliveryCount answers GET /api/delivery/count: the band's light number.
func (s *Server) apiDeliveryCount(w http.ResponseWriter, _ *http.Request) {
	count := 0
	if s.deliver != nil {
		count = s.deliver.DeliveryCount()
	}
	writeJSON(w, http.StatusOK, struct {
		Count int `json:"count"`
	}{Count: count})
}

// apiDeliver answers POST /api/delivery: the selected members, by herdr name.
func (s *Server) apiDeliver(w http.ResponseWriter, r *http.Request) {
	if s.deliver == nil {
		http.Error(w, t("web.error.not_found"), http.StatusNotFound)
		return
	}
	in, ok := s.apiRead(w, r)
	if !ok {
		return
	}
	answer, err := s.deliver.Deliver(in.Members)
	if err != nil {
		apiAnswer(w, nil, refuse(http.StatusUnprocessableEntity, err.Error()))
		return
	}
	apiAnswer(w, answer, nil)
}
