package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

type slideInfo struct {
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Caption string `json:"caption,omitempty"`
}
type materials struct {
	Headline    string      `json:"headline"`
	Description string      `json:"description"`
	Slides      []slideInfo `json:"slides,omitempty"`
}

type insight struct {
	Rule     string `json:"rule"`
	Source   string `json:"source"`
	Evidence string `json:"evidence"`
	Why      string `json:"why"`
	Action   string `json:"action"`
	Verify   string `json:"verify"`
}

func validInsights(list []insight, m materials) bool {
	if len(list) == 0 || len(list) > 4 {
		return false
	}
	images := map[string]bool{"Обложка": true}
	for i := range m.Slides {
		images[fmt.Sprintf("Слайд %d", i+2)] = true
	}
	for _, item := range list {
		for _, s := range []string{item.Rule, item.Source, item.Evidence, item.Why, item.Action, item.Verify} {
			if strings.TrimSpace(s) == "" {
				return false
			}
		}
		if images[item.Source] {
			continue
		}
		text := map[string]string{"Описание": m.Description, "Заголовок": m.Headline}[item.Source]
		if !strings.Contains(text, item.Evidence) {
			return false
		}
	}
	return true
}

type finding struct {
	ID       string `json:"id"`
	Rule     string `json:"rule"`
	Status   string `json:"status"` // fix, ok, unknown
	Priority string `json:"priority"`
	Source   string `json:"source"`
	Evidence string `json:"evidence"`
	Why      string `json:"why"`
	Action   string `json:"action"`
	Verify   string `json:"verify"`
}

const osObject = `(?:операционн[а-яё]+\s+систем[а-яё]*|ос|windows|linux)`
const install = `(?:необходимо|нужно|следует|потребуется|прид[её]тся)\s+(?:самостоятельно\s+)?установить`

var setupText = regexp.MustCompile(`(?i)(?:` + install + `\s+` + osObject + `|установите\s+` + osObject + `|требуется\s+установка\s+` + osObject + `|(?:` + osObject + `|е[её])\s+` + install + `)(?:$|[^а-яёa-z])`)
var systemWord = regexp.MustCompile(`(?i)(операционн[а-яё]*\s+систем[а-яё]*|(?:^|[^а-яёa-z])ос(?:$|[^а-яёa-z])|windows|linux)`)
var unsupportedClaim = regexp.MustCompile(`(?i)гарантированно лучший|идеален для всех|самый мощный|для любых задач`)
var bundledSystem = regexp.MustCompile(`(?i)(?:с\s+предустановленной\s+(?:ос\s+)?|предустановлена\s+(?:ос\s+)?)(?:windows\s*(?:10|11)?|linux)`)

func contradictingOS(m materials, f fact) string {
	if !strings.EqualFold(f.Value, "без ОС") && !strings.EqualFold(f.Value, "noOS") {
		return ""
	}
	return bundledSystem.FindString(m.Description)
}

func excerpt(s string) string {
	const max = 220
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max]) + "…"
}

func analyze(r report, profile productProfile) []finding {
	m := r.Materials
	rows := []finding{}
	for _, field := range [][2]string{{"model", "Модель"}, {"sku", "Артикул"}, {"cpu", "Процессор"}, {"ram", "ОЗУ"}, {"ssd", "SSD"}, {"os", "ОС"}} {
		f := profile[field[0]]
		if f.Alternative != "" && !strings.EqualFold(f.Value, f.Alternative) {
			rows = append(rows, finding{"fact-" + field[0], "Согласованность данных", "fix", "Высокий", "Профиль · " + field[1], f.Value + " (" + f.Source + ") / " + f.Alternative + " (" + f.OtherSource + ")", "Покупателю нужны характеристики именно этой поставки.", "Сверьте обе записи и укажите подтверждённое значение.", "Проверить значение по артикулу в источнике продавца."})
		}
	}
	osRule := finding{"os-setup", "Подготовка к первому запуску", "unknown", "Высокий", "Профиль · ОС", "Нет подтверждения поставки без ОС.", "От комплектации зависит, что нужно сделать после покупки.", "Уточните комплектацию по артикулу.", "Попросите покупателя назвать первый шаг после распаковки."}
	f := profile["os"]
	conflict := contradictingOS(m, f)
	switch {
	case conflict != "":
		osRule.Rule = "Комплектация и первый запуск"
		osRule.Source = "Профиль · ОС и описание"
		osRule.Status = "fix"
		osRule.Evidence = f.Value + " (" + f.Source + ") / «" + conflict + "»"
		osRule.Action = "Сверьте комплектацию по артикулу. До подтверждения исключите обещания о системе из макета."
		rows = append(rows, osRule)
	case f.Value == "" || (f.Alternative != "" && !strings.EqualFold(f.Value, f.Alternative)):
		rows = append(rows, osRule)
	case strings.EqualFold(f.Value, "без ОС") || strings.EqualFold(f.Value, "noOS"):
		osRule.Evidence = f.Value + " (" + f.Source + ")"
		osRule.Action = "Объясните, что до начала работы потребуется установить операционную систему."
		if r.Brief != nil && r.Brief.Answers["occasion"] == "Для учёбы" {
			osRule.Action = "Добавьте пояснение: «Операционная система не установлена. Перед первым занятием её нужно установить»."
		}
		type fragment struct{ source, text string }
		fragments := []fragment{{"Заголовок предложения", m.Headline}, {"Описание", m.Description}}
		for i, s := range m.Slides {
			fragments = append(fragments, fragment{fmt.Sprintf("Слайд %d · подпись продавца", i+2), s.Caption})
		}
		if m.Description != "" {
			osRule.Status = "fix"
			osRule.Source += " + переданные тексты"
			osRule.Evidence += ". В проверенных текстах явная инструкция по установке не найдена."
		} else {
			osRule.Evidence += ". Описание не передано."
		}
		for _, part := range fragments {
			// Keep the instruction and the OS mention in the same local passage.
			for _, line := range strings.Split(part.text, "\n") {
				for _, pos := range setupText.FindAllStringIndex(line, -1) {
					before := []rune(line[:pos[0]])
					after := []rune(line[pos[1]:])
					around := string(before[max(0, len(before)-80):]) + line[pos[0]:pos[1]] + string(after[:min(80, len(after))])
					if !systemWord.MatchString(around) || strings.HasSuffix(" "+strings.ToLower(strings.TrimSpace(line[:pos[0]])), " не") {
						continue
					}
					osRule.Status = "ok"
					osRule.Source = part.source
					osRule.Evidence = around
					osRule.Action = "Сохраните пояснение о подготовке."
					break
				}
				if osRule.Status == "ok" {
					break
				}
			}
			if osRule.Status == "ok" {
				break
			}
		}
		rows = append(rows, osRule)
	}
	task := ""
	if r.Brief != nil {
		task = r.Brief.Answers["task"]
	}
	context := finding{"buyer-task", "Задача покупателя", "unknown", "Средний", "Анкета · задача", "Конкретная задача не указана.", "Сценарий определяет, какую пользу показать первой.", "Укажите одно действие покупателя после получения товара.", "Сопоставьте главный тезис со сценарием."}
	if task != "" {
		context.Status = "ok"
		context.Evidence = task
		context.Action = "Используйте этот сценарий для выбора примера."
	}
	rows = append(rows, context)
	for _, part := range [][3]string{{"headline", "Заголовок предложения", m.Headline}, {"description", "Описание", m.Description}} {
		v := finding{part[0], "Ясность текста", "unknown", "Средний", part[1], "Материал не передан.", "Покупателю нужны проверяемые сведения для выбора.", "Передайте исходный текст для проверки.", "Сверьте тезис с профилем товара."}
		if part[2] != "" {
			v.Status = "ok"
			v.Evidence = excerpt(part[2])
			v.Rule = "Проверка рекламных формулировок"
			v.Action = "По словарю проверки совпадений нет."
			if phrase := unsupportedClaim.FindString(part[2]); phrase != "" {
				v.Status = "fix"
				v.Evidence = phrase
				v.Action = "Замените общее обещание конкретной задачей и подтверждённым свойством."
			}
		}
		rows = append(rows, v)
	}
	model := profile["model"]
	if model.Value != "" && (model.Alternative == "" || strings.EqualFold(model.Alternative, model.Value)) && m.Headline != "" {
		match := finding{"headline-model", "Узнаваемость модели", "ok", "Средний", "Заголовок предложения", excerpt(m.Headline), "Точное обозначение помогает сравнивать одну конфигурацию.", "Название модели совпадает с профилем.", "Сверьте название с артикулом поставки."}
		if !strings.Contains(strings.ToLower(m.Headline), strings.ToLower(model.Value)) {
			match.Status = "fix"
			match.Action = "Проверьте обозначение модели: точная строка «" + model.Value + "» из профиля не найдена в заголовке."
		}
		rows = append(rows, match)
	}
	for i, c := range r.Checks {
		status := "ok"
		if !c.OK {
			status = "fix"
		}
		rows = append(rows, finding{fmt.Sprintf("cover-%d", i), c.Label, status, "Средний", "Обложка", c.Note, "Размер и вес влияют на варианты показа миниатюры.", c.Note, "Проверить следующую выгрузку по тому же ориентиру."})
	}
	for i, s := range m.Slides {
		status, action := "ok", "Сохраните исходное разрешение."
		if min(s.Width, s.Height) < 800 {
			status = "fix"
			action = "Экспортируйте слайд из исходника большего размера."
		}
		rows = append(rows, finding{fmt.Sprintf("gallery-%d", i+2), "Разрешение слайда", status, "Средний", fmt.Sprintf("Слайд %d", i+2), fmt.Sprintf("%d × %d px", s.Width, s.Height), "Мелкие детали потребуют проверки в миниатюре.", action, "Проверить короткую сторону по ориентиру 800 px."})
	}
	return rows
}
