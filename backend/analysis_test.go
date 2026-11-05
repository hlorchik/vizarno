package main

import (
	"strings"
	"testing"
)

func TestSetupEvidence(t *testing.T) {
	profile := productProfile{"os": {Value: "без ОС", Source: "Предложение продавца"}}
	for _, tc := range []struct{ description, caption, status, source string }{
		{"", "", "unknown", "Профиль"},
		{"Без ОС.", "", "fix", "Профиль"},
		{"Без ОС. Нужно установить программу для видеоурока.", "", "fix", "Профиль"},
		{"Не нужно установить ОС.", "", "fix", "Профиль"},
		{"Перед уроком нужно установить ОС.", "", "ok", "Описание"},
		{"Без ОС.", "Операционная система не установлена. Перед первым занятием её нужно установить.", "ok", "Слайд 2"},
	} {
		r := report{Materials: materials{Description: tc.description, Slides: []slideInfo{{800, 1000, tc.caption}}}}
		var match finding
		for _, f := range analyze(r, profile) {
			if f.ID == "os-setup" {
				match = f
			}
		}
		if match.Status != tc.status || !strings.HasPrefix(match.Source, tc.source) {
			t.Errorf("%q, %q: %+v", tc.description, tc.caption, match)
		}
	}
}

func TestReviewSources(t *testing.T) {
	m := materials{Headline: "MateBook", Description: "ОС не установлена", Slides: []slideInfo{{800, 1000, ""}}}
	for _, tc := range []struct {
		source, quote string
		valid         bool
	}{
		{"Описание", "не установлена", true}, {"Заголовок", "не установлена", false}, {"Слайд 2", "Товар в центре", true},
		{"Слайд 3", "Товар в центре", false}, {"Обложка", "", false},
	} {
		result := validInsights([]insight{{"Читаемость", tc.source, tc.quote, "Найти модель", "Проверить подпись", "Прочитать в миниатюре"}}, m)
		if result != tc.valid {
			t.Errorf("unexpected validation: %s %q", tc.source, tc.quote)
		}
	}
}

func TestRetailerExamples(t *testing.T) {
	profile := productProfile{"os": {Value: "без ОС", Source: "Характеристики, 53013YDL"}}
	for _, sample := range []struct {
		seller, description string
		conflict            bool
	}{
		{"Lite Mobile", "Операционная система: без ОС", false},
		{"iQ", "Поступает с предустановленной ОС Windows 11.", true},
	} {
		t.Run(sample.seller, func(t *testing.T) {
			r := report{Materials: materials{Description: sample.description}}
			text := planLayout("MateBook D 16", 1, 1, profile, r).text()
			if strings.Contains(text, "профиль расходится с описанием") != sample.conflict {
				t.Fatal("ошибка сверки источников")
			}
			if strings.Contains(text, "Без ОС") == sample.conflict {
				t.Fatal("ошибка выбора текста для макета")
			}
		})
	}
}
