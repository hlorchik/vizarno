package main

import "testing"

func TestMethodRequiresEveryApplicableDecision(t *testing.T) {
	rules := releaseRules()
	if len(rules) != 38 {
		t.Fatalf("expected 38 replacement rules, got %d", len(rules))
	}
	all := make([]methodDecision, len(rules))
	for i, r := range rules {
		all[i] = methodDecision{Rule: i + 1, Status: "pass", Slide: 1, Note: "Основание на слайде"}
		if r.Conditional {
			all[i].Status = "skip"
			all[i].Slide = 0
		}
	}
	for _, tc := range []struct {
		name      string
		change    func([]methodDecision) []methodDecision
		ok, valid bool
	}{
		{"complete", func(c []methodDecision) []methodDecision { return c }, true, true},
		{"no verdict", func(c []methodDecision) []methodDecision { return c[:len(c)-1] }, false, false},
		{"duplicate", func(c []methodDecision) []methodDecision { c[1].Rule = 1; return c }, false, false},
		{"mandatory skipped", func(c []methodDecision) []methodDecision { c[0].Status = "skip"; return c }, false, false},
		{"unknown slide", func(c []methodDecision) []methodDecision { c[0].Slide = 6; return c }, false, false},
		{"no evidence", func(c []methodDecision) []methodDecision { c[0].Note = ""; return c }, false, false},
		{"missing", func(c []methodDecision) []methodDecision { c[0].Status = "missing"; return c }, false, true},
		{"fix", func(c []methodDecision) []methodDecision { c[0].Status = "fix"; return c }, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ok, err := evaluateDecisions(tc.change(append([]methodDecision(nil), all...)), rules, 2)
			if ok != tc.ok || (err == nil) != tc.valid {
				t.Fatal(ok, err)
			}
		})
	}
}

func TestLocalReleaseReview(t *testing.T) {
	report := localReleaseReview("wildberries", 8)
	if !report.Approved || report.Kind != "local" || len(report.Checks) != 6 {
		t.Fatalf("unexpected local report: %#v", report)
	}
	for _, check := range report.Checks {
		if check.Status != "pass" || check.Note == "" {
			t.Fatalf("incomplete local check: %#v", check)
		}
	}
}

func TestPlanUsesConfirmedBenefit(t *testing.T) {
	b := &brief{Answers: map[string]string{"buyer": "Близкий человек", "occasion": "Для учёбы", "priority": "Цена", "channel": "В мобильной выдаче", "visual": "Отдельный предмет", "task": "Подготовиться к первому уроку", "concern": "Успеем ли подготовить ноутбук?", "barriers": "Нужно установить систему", "keyCriteria": "Экран, память и накопитель", "motive": "Успеть к началу", "message": "Подготовьте систему к уроку", "basis": "Беседа с продавцом", "benefitField": "Операционная система", "advantage": "Можно установить нужную систему", "benefit": "Подготовьте программы для занятий", "proof": "Комплектация поставщика"}}
	b.Answers["endUser"] = "Школьник"
	b.Answers["experience"] = "Первый ноутбук"
	profile := productProfile{"model": {Value: "D16", Source: "Паспорт"}, "sku": {Value: "53013YDL", Source: "Паспорт"}, "cpu": {Value: "i5", Source: "Паспорт"}, "ram": {Value: "16 ГБ", Source: "Паспорт"}, "ssd": {Value: "512 ГБ", Source: "Паспорт"}, "display": {Value: "16 дюймов", Source: "Паспорт"}, "os": {Value: "без ОС", Source: "Комплектация"}}
	p := planLayout("", 1, 1, profile, report{Brief: b})
	if len(p.ReleaseNeeds) != 0 || len(p.Steps) != 8 || len(p.Steps[0].Copy) != 3 || p.Steps[1].Copy[0] != b.Answers["benefit"] {
		t.Fatal(p)
	}
	profile["os"] = fact{Value: "без ОС", Source: "Комплектация", Alternative: "Windows", OtherSource: "Описание"}
	p = planLayout("", 1, 1, profile, report{Brief: b})
	if len(p.ReleaseNeeds) == 0 {
		t.Fatal("released a conflicting property", p)
	}
}
