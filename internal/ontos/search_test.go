package ontos

import (
	"reflect"
	"testing"
)

func titles(hits []Summary) []string {
	out := []string{}
	for _, h := range hits {
		out = append(out, h.Title)
	}
	return out
}

func searchStore(t *testing.T) *Store {
	t.Helper()
	s := newStore(t, "")
	mustSubject(t, s, &Subject{Name: "app"})
	mustSubject(t, s, &Subject{Name: "lib"})
	mustEntry(t, s, Entry{Title: "Broker reconnect", Category: "mechanism", Subjects: []string{"app"}, Tags: []string{"broker"},
		Body: "the client resubscribes"})
	mustEntry(t, s, Entry{Title: "Startup order", Category: "mechanism", Subjects: []string{"app"},
		Body: "settings, auth, then the broker connects"})
	mustEntry(t, s, Entry{Title: "Subscribe while disconnected", Category: "gotcha", Subjects: []string{"lib"}, Tags: []string{"broker"},
		Trigger: "before calling subscribe on the broker client", Body: "it silently does nothing"})
	return s
}

func TestSearchRequiresEveryTerm(t *testing.T) {
	s := searchStore(t)
	if got := titles(s.Search("broker RESUBSCRIBES", Filter{})); !reflect.DeepEqual(got, []string{"Broker reconnect"}) {
		t.Errorf("got %v, want only the entry holding both terms", got)
	}
	if got := s.Search("broker nowhere", Filter{}); len(got) != 0 {
		t.Errorf("got %v, want nothing: one term matches no entry", titles(got))
	}
}

func TestSearchRanksTitleAboveBody(t *testing.T) {
	s := searchStore(t)
	// "broker": title (3) + body 0 → 3; trigger (2) + body 0 → 2; body only → 1.
	want := []string{"Broker reconnect", "Subscribe while disconnected", "Startup order"}
	if got := titles(s.Search("broker", Filter{})); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestSearchFiltersCombineWithAnd(t *testing.T) {
	s := searchStore(t)
	cases := []struct {
		f    Filter
		want []string
	}{
		{Filter{Tag: "broker"}, []string{"Broker reconnect", "Subscribe while disconnected"}},
		{Filter{Tag: "broker", Subject: "app"}, []string{"Broker reconnect"}},
		{Filter{Tag: "broker", Subject: "app", Category: "gotcha"}, []string{}},
		// No query: category order (mechanism before gotcha), then title.
		{Filter{}, []string{"Broker reconnect", "Startup order", "Subscribe while disconnected"}},
	}
	for _, c := range cases {
		if got := titles(s.Search("", c.f)); !reflect.DeepEqual(got, c.want) {
			t.Errorf("filter %+v: got %v, want %v", c.f, got, c.want)
		}
	}
}

func TestSearchCheckRejectsUnknownNames(t *testing.T) {
	s := searchStore(t)
	if err := s.Check(Filter{Category: "maps"}); err == nil {
		t.Error("an unknown category was accepted")
	}
	if err := s.Check(Filter{Subject: "nope"}); err == nil {
		t.Error("an unknown subject was accepted")
	}
}
