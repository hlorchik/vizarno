package main

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestStudyBrief(t *testing.T) {
	form := url.Values{"questionnaire": {"yes"}, "buyer": {"Близкий человек"}, "occasion": {"Для учёбы"},
		"priority": {"Цена"}, "channel": {"В мобильной выдаче"}, "visual": {"Отдельный предмет"}, "task": {"Подключиться к видеоуроку утром"},
		"benefitField": {"Операционная система"}}
	request := httptest.NewRequest("POST", "/", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	b, err := makeBrief(request)
	if err != nil {
		t.Fatal(err)
	}
	profile := productProfile{"os": {Value: "без ОС", Source: "комплектация продавца"}}
	plan := planLayout("Ноутбук", 1, 2, profile, report{Brief: b})
	if !strings.Contains(plan.Context, form.Get("task")) || !strings.Contains(plan.text(), "Перед началом работы потребуется установить операционную систему.") {
		t.Fatal("утрачены сценарий или пояснение до занятия")
	}
	delete(profile, "os")
	if strings.Contains(planLayout("Ноутбук", 1, 2, profile, report{Brief: b}).text(), "Без ОС") {
		t.Fatal("неподтверждённая ОС попала в ТЗ")
	}
}

func TestDraftCoverAndSellerQuestions(t *testing.T) {
	plan := planLayout("Ноутбук", 1, 1, nil, report{})
	plan.Tasks = []finding{{Status: "unknown", Source: "Описание", Action: "Передайте описание."}}
	text := plan.text()
	if len(plan.Steps[0].Copy) != 0 || strings.Contains(text, "Выбранные правки") || !strings.Contains(text, "Текст для макета пока не согласован.") ||
		!strings.Contains(strings.Split(text, "Уточнить у продавца")[1], "Передайте описание.") {
		t.Fatal(text)
	}
	profile := productProfile{"model": {Value: "HUAWEI MateBook D 16", Source: "Паспорт изделия"}}
	cover := planLayout("Ноутбук", 1, 1, profile, report{}).Steps[0]
	if len(cover.Copy) != 3 || cover.Copy[1] != profile["model"].Value || cover.Copy[2] != "Покупай!" {
		t.Fatal(cover)
	}
}
